# Decisiones de arquitectura

Una decisión por sección, cada una con su trade-off.

## El sandbox del filesystem asume symlinks y dotfiles hostiles, no solo `..`

**Decisión:** `resolvePath` (`internal/tools/fs.go`) rechaza tres cosas, no
solo una: un lexical climb con `..`, un componente de path que empiece con
`.` (bloquea `.env`, `.git`, etc.), y un symlink dentro de `-dir` que apunte
fuera de él (resuelto con `filepath.EvalSymlinks` antes de comparar contra
el workdir resuelto).

**Trade-off:** el modelo no tiene forma de crear un symlink desde su JS, pero
sí puede pedirle a `readFile`/`readCsv` que siga uno que ya exista en el
directorio — y el riesgo concreto de los dotfiles es peor de lo que parece:
si alguien corre el agente con `-dir .` (el default) desde el mismo
directorio donde vive su `.env`, el modelo podría hacer `readFile(".env")` y
mandar la API key de vuelta al proveedor del LLM como parte de la
conversación. Bloquear dotfiles por completo es más simple que mantener una
lista de nombres de archivo sensibles, y no cuesta nada real: un flujo de
reconciliación no necesita leer archivos ocultos.

## El timeout de `Run` no puede quedar bloqueado por una tool colgada

**Decisión:** al vencer el timeout, `Run` llama a `vm.Interrupt()` y espera
un `interruptGrace` (2s) adicional por la goroutine que ejecuta el JS — no
espera indefinidamente. El buffer de output (`safeBuffer`) usa un mutex
porque, si la goroutine sigue viva después de ese grace period, todavía
puede estar escribiendo en él.

**Trade-off:** `vm.Interrupt()` es casi instantáneo para un loop infinito en
JS puro (goja chequea el interrupt entre operaciones), pero no puede
interrumpir una llamada bloqueada dentro de una tool de Go (por ejemplo, un
`readFile` sobre un archivo enorme o un pipe). Sin el grace period, una tool
colgada bloquearía `Run` — y por lo tanto todo el loop del agente —
indefinidamente, pese al timeout configurado. El costo es aceptar una
goroutine "leaked" en ese caso raro (vive hasta que la llamada bloqueada
termine por su cuenta); para un PoC es preferible a que el proceso completo
se cuelgue.

## goja en vez de Docker+Python o `go run`

**Decisión:** ejecutar el código del modelo con
[goja](https://github.com/dop251/goja), un intérprete de ES5.1+ escrito en Go
puro, embebido en el mismo proceso.

**Trade-off:** Docker+Python (o cualquier sandbox por subproceso) da un
sandbox de sistema operativo real y acceso al ecosistema completo de
librerías de Python, pero agrega una dependencia externa (el daemon de
Docker), latencia de arranque por contenedor, y superficie de ataque de
gestión de procesos (matar el contenedor si se cuelga, limpiar volúmenes,
etc.). `go run` sobre un archivo temporal compilaría Go arbitrario del
modelo — inaceptable como sandbox, y con latencia de compilación en cada
turno. goja da un sandbox real (la VM solo tiene lo que registramos
explícitamente: sin `require`, sin red, sin filesystem) sin proceso externo,
arranca en microsegundos, y corre en cualquier máquina que corra el binario
de Go — sin runtime de Node, sin Python, sin CGO. El costo: JS ES5.1 es más
limitado que Python para análisis de datos (no hay pandas), y goja no
implementa el 100% de motores V8 modernos (sin async/await, sin muchas APIs
de Node). Para el caso de uso — leer CSVs, iterar, agrupar — ES5.1 con
Array.prototype es suficiente.

## VM nueva por ejecución, no reutilizada entre turnos

**Decisión:** `Runtime.Run` llama `goja.New()` en cada invocación.

**Trade-off:** reutilizar la VM entre turnos ahorraría el costo (pequeño) de
crear una VM nueva, y permitiría que el modelo "recuerde" variables entre
bloques de código sin tener que volver a leer los CSVs. Pero eso acopla la
ejecución al historial de la conversación de una forma implícita y frágil:
si un turno anterior falló a mitad de camino y dejó variables a medio
definir, el siguiente intento hereda ese estado contaminado, y el modelo no
tiene visibilidad de qué quedó definido. Una VM nueva por turno hace que
cada intento sea determinista dado su código — el único estado que persiste
es el que el modelo puede ver (el historial de mensajes), nunca un efecto de
lado invisible en el intérprete. El costo es que el modelo debe releer los
CSVs si necesita el mismo dato en dos turnos distintos; para el tamaño de
datos de este caso de uso eso es barato.

## El error del JS vuelve en `Result.Err`, no como error de Go

**Decisión:** `Run(code string) Result` nunca devuelve `(Result, error)` — un
syntax error, una excepción lanzada, o un timeout son parte de `Result`, no
un error de la función Go.

**Trade-off:** semánticamente, un error de Go (`error` como segundo valor de
retorno) comunica "esta operación de Go falló". Pero un error de JS no es una
falla del programa Go — es información de dominio: es exactamente lo que el
modelo necesita ver para corregir su código en el siguiente turno. Si `Run`
devolviera `error`, el loop del agente tendría que hacer un cast especial en
cada call site para decidir "esto es un error real de infraestructura vs.
esto es una observación que debo mandarle al modelo" — dos casos que en la
práctica se tratan igual (se imprimen y se mandan como observación). Modelar
el error de JS como parte del resultado hace ese caso el camino normal, no
una excepción a manejar.

## El channel del timeout es buffered

**Decisión:** `done := make(chan error, 1)` en `Runtime.Run`.

**Trade-off:** con un channel sin buffer, si el `select` elige la rama del
timeout, la goroutine que corre `vm.RunString` eventualmente termina (cuando
el intérprete atiende el `vm.Interrupt()`) e intenta hacer `done <- err`.
Sin buffer, ese send bloquea para siempre porque ya nadie está leyendo del
channel — la goroutine queda leaked hasta que el proceso termine. Con
buffer de 1, el send siempre tiene espacio, la goroutine termina limpio, y
el `Run` que ganó por timeout puede drenar el channel (`<-done`) para
confirmar que efectivamente terminó antes de devolver el resultado.

## `panic` dentro de las tools, no `(valor, error)`

**Decisión:** cada función Go registrada en la VM que puede fallar
(`readCsv`, `readFile`, etc.) está envuelta en un closure que hace
`panic(vm.ToValue(err.Error()))` en vez de devolver `(T, error)` directamente
a goja.

**Trade-off:** si se registra una función Go con firma `(T, error)`
directamente, goja la expone a JS como un array de dos elementos
`[valor, error]` — el modelo tendría que escribir
`var result = readCsv(...); if (result[1]) { ... }` en cada llamada, un
patrón que no es JS idiomático y que el modelo tiende a olvidar. Un panic
dentro de una función host de goja se convierte automáticamente en una
excepción JS normal, atrapable con `try/catch` — el patrón que cualquier
LLM entrenado en JS ya conoce y usa de forma consistente. El costo es que
"panic" suena alarmante en Go idiomático fuera de este contexto; el
comentario en el código documenta por qué es intencional aquí.

## La interface `Provider`, no un cliente por proveedor sin abstracción

**Decisión:** `internal/llm.Provider` es una interface con un solo método
(`Complete`), implementada por `AnthropicProvider` y `OpenAIProvider`; el
resto del código (el loop del agente, main) solo conoce la interface.

**Trade-off:** sin esta interface, agregar o cambiar de proveedor requeriría
tocar el loop del agente. Con ella, `internal/agent` no sabe ni le importa
si está hablando con Anthropic, OpenAI, OpenRouter o Ollama — el router
(`internal/llm/router.go`) decide la implementación concreta a partir de
env vars, en tiempo de ejecución, sin recompilar. El costo de la interface
es la indirección de una llamada de método extra y un archivo más — mínimo,
y justificado porque el requisito explícito del proyecto es "cualquier
provider vía configuración, sin recompilar".

## Qué pasaría si esto fuera a producción, y qué falta

Esto es un prototipo de un solo usuario, de un solo proceso, sin persistencia
más allá de los archivos que el propio código JS escribe. Para producción
faltaría, como mínimo:

- **Aislamiento real de recursos.** goja limita el sandbox a "qué funciones
  puede llamar", pero no limita CPU ni memoria dentro del intérprete — un
  bucle que asigna memoria sin parar puede agotar el proceso antes de que
  el timeout de tiempo lo corte. Necesitaría límites de memoria (o mover la
  ejecución a un proceso/worker separado con cgroups).
- **Persistencia del historial.** El historial vive en memoria del proceso
  (`Loop.history`); un reinicio pierde la conversación. Producción necesita
  guardar el historial (y el código ejecutado) en almacenamiento durable,
  tanto para continuidad como para auditoría.
- **Auditoría y observabilidad.** Cada bloque de código ejecutado debería
  quedar logueado con su input, output, duración y qué archivos tocó —
  crítico en un dominio financiero donde alguien puede necesitar reconstruir
  qué vio y decidió el agente.
- **Multi-tenancy.** El flag `-dir` asume un solo usuario con un solo
  working directory. Con múltiples usuarios, cada sesión necesita su propio
  workdir aislado y su propia cuota de ejecución, no un flag global.
- **Reintentos y rate limiting del lado del `Provider`.** Ahora un fallo de
  red o una respuesta vacía del LLM (ver el "Nota" en el README sobre el
  modelo gratuito usado en el demo) se propaga como error del turno. En
  producción, el router debería reintentar automáticamente errores
  transitorios antes de dárselos al usuario.
- **Límites explícitos sobre qué puede escribir `writeFile`.** Hoy cualquier
  archivo dentro de `-dir` es escribible por el código que el modelo genera.
  Para datos financieros reales, probablemente se querría que la escritura
  sea opt-in por comando, o de solo lectura por default.
