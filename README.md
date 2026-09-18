# codeact

Agente de reconciliación financiera que responde preguntas en lenguaje
natural sobre archivos CSV escribiendo y ejecutando JavaScript, en vez de
emitir una tool call de JSON por turno.

## Por qué

Un join entre dos CSVs seguido de un group-by no cabe en una tool call plana:
necesitas un loop, una variable intermedia (un índice por referencia) y una
condición de filtrado. Eso son ~15 líneas de JS. Forzar ese mismo trabajo a
través de llamadas a herramientas discretas (`readCsv`, luego `filter`, luego
`groupBy`, cada una una ronda de ida y vuelta con el modelo) es más lento,
más frágil y no compone. CodeAct le da al modelo un intérprete: escribe
código, lo corremos, el resultado (stdout o el error) vuelve como la
observación del siguiente turno, y el modelo corrige su propio código si
falló.

## Estructura

```
cmd/agent/main.go       CLI: flags, .env, REPL
internal/llm/           interface Provider + Anthropic + OpenAI-compatible + router
internal/runtime/       wrapper de goja: ejecución con timeout, captura de output
internal/tools/         readCsv, listFiles, readFile, writeFile — todo scoped a -dir
internal/agent/         el loop think→code→execute→observe, system prompt, extracción de código
testdata/               banco.csv / ledger.csv de ejemplo para el demo end-to-end
```

## Cómo se construye y corre

Requiere Go 1.23+. La única dependencia externa es
[`github.com/dop251/goja`](https://github.com/dop251/goja) (intérprete de JS
en Go puro, sin CGO). Todo lo demás es stdlib.

```bash
go build -o codeact ./cmd/agent
go test ./...
```

Copia `.env.example` a `.env` y pon tus credenciales (ver la sección de
configuración abajo), luego:

```bash
./codeact -dir testdata
```

Flags:

- `-dir` — directorio de trabajo con los CSVs (default `.`). Todas las
  tools (`readCsv`, `listFiles`, `readFile`, `writeFile`) están limitadas a
  este directorio; cualquier path que intente escapar (`../..`) es
  rechazado.
- `-timeout` — timeout de ejecución de JS por bloque de código (default
  `10s`).

El REPL lee preguntas de stdin. Escribe `exit` o `quit` para salir.

## Configuración del provider

El agente habla con cualquier proveedor LLM vía variables de entorno, sin
recompilar. `LLM_PROVIDER` selecciona el formato de API:

- `anthropic` — Anthropic Messages API (`/v1/messages`).
- `openai` — cualquier endpoint compatible con OpenAI chat/completions:
  OpenAI, OpenRouter, Groq, Together, DeepSeek, Ollama... Solo cambia
  `LLM_BASE_URL` y `LLM_MODEL`.

### Ejemplo: Anthropic directo

```env
LLM_PROVIDER=anthropic
LLM_BASE_URL=
LLM_MODEL=claude-sonnet-4-5
LLM_API_KEY=sk-ant-...
```

### Ejemplo: OpenRouter

```env
LLM_PROVIDER=openai
LLM_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=anthropic/claude-sonnet-4.5
LLM_API_KEY=sk-or-...
LLM_HTTP_REFERER=https://example.com
LLM_TITLE=codeact-agent
```

`LLM_HTTP_REFERER` y `LLM_TITLE` son opcionales — OpenRouter los usa para
atribuir uso, y se ignoran sin problema contra cualquier otro endpoint
OpenAI-compatible.

Si falta `LLM_PROVIDER`, `LLM_MODEL` o `LLM_API_KEY`, el binario imprime un
error claro y sale con código 1 (nunca panic).

## Cómo funciona code-as-action

En cada turno:

1. El input del usuario se agrega al historial.
2. Se llama al `Provider` con el system prompt (que documenta las tools
   disponibles) y el historial completo.
3. Se busca un bloque ` ```js ` en la respuesta.
   - Si no hay bloque, esa respuesta es la respuesta final — se muestra al
     usuario y el turno termina.
   - Si hay un bloque, se ejecuta en una VM de goja nueva (sandbox: solo
     tiene lo que registramos — `print`, `readCsv`, `listFiles`, `readFile`,
     `writeFile`; sin `require`, sin red, sin filesystem fuera de `-dir`).
4. El output (o el error) del código se agrega al historial como un mensaje
   de usuario — es la "observación" que el modelo lee en el siguiente turno
   para decidir si ya tiene la respuesta o necesita corregir su código.
5. Esto se repite hasta 8 veces por turno (`maxIterations` en
   `internal/agent/loop.go`); si el modelo nunca da una respuesta en texto
   plano, el turno falla explícitamente en vez de colgarse.

Esto es lo que permite que un join + group-by salga en una sola acción: el
modelo no necesita una tool `joinAndGroupBy` — escribe el loop él mismo.

## Verificación end-to-end

`testdata/banco.csv` y `testdata/ledger.csv` tienen 20 filas cada uno
(`id,reference,amount,date`). Tres referencias del banco no tienen
contraparte en el ledger:

| referencia | fecha | monto |
|---|---|---|
| TXN005 | 2024-03-01 | 500.00 |
| TXN012 | 2024-03-03 | 275.00 |
| TXN018 | 2024-03-04 | 95.75 |

(El ledger también tiene 3 referencias propias sin contraparte en el banco —
`LED101`, `LED102`, `LED103` — como distractor; no deberían aparecer en la
respuesta a "transacciones del banco sin contraparte en el ledger".)

Corrida real contra OpenRouter (`nvidia/nemotron-3-ultra-550b-a55b:free`),
pregunta: *"Encuéntrame las transacciones del banco sin contraparte en el
ledger, agrupadas por día, con el monto total."*

```
codeact agent ready (provider=openai, dir=testdata). Type a question, or 'exit' to quit.

>
--- code ---
var files = listFiles(".");
print(files);
--- output ---
banco.csv,ledger.csv

--- code ---
var bank = readCsv("banco.csv");
var ledger = readCsv("ledger.csv");

print("Bank columns:", Object.keys(bank[0] || {}));
print("Ledger columns:", Object.keys(ledger[0] || {}));
print("Bank sample:", bank.slice(0, 3));
print("Ledger sample:", ledger.slice(0, 3));
--- output ---
Bank columns: id,reference,amount,date
Ledger columns: id,reference,amount,date
Bank sample: [object Object],[object Object],[object Object]
Ledger sample: [object Object],[object Object],[object Object]

--- code ---
var bank = readCsv("banco.csv");
var ledger = readCsv("ledger.csv");

print("Bank rows:", bank.length);
print("Ledger rows:", ledger.length);
print("First bank row:", JSON.stringify(bank[0]));
print("First ledger row:", JSON.stringify(ledger[0]));
--- output ---
Bank rows: 20
Ledger rows: 20
First bank row: {"id":"1","reference":"TXN001","amount":"150.00","date":"2024-03-01"}
First ledger row: {"id":"1","reference":"TXN001","amount":"150.00","date":"2024-03-01"}

--- code ---
var bank = readCsv("banco.csv");
var ledger = readCsv("ledger.csv");

var ledgerRefs = {};
for (var i = 0; i < ledger.length; i++) {
  ledgerRefs[ledger[i].reference] = true;
}

var unmatched = bank.filter(function (row) {
  return !ledgerRefs[row.reference];
});

print("Unmatched count:", unmatched.length);
print("Unmatched transactions:");
for (var j = 0; j < unmatched.length; j++) {
  print(JSON.stringify(unmatched[j]));
}
--- output ---
Unmatched count: 3
Unmatched transactions:
{"id":"5","reference":"TXN005","amount":"500.00","date":"2024-03-01"}
{"id":"12","reference":"TXN012","amount":"275.00","date":"2024-03-03"}
{"id":"18","reference":"TXN018","amount":"95.75","date":"2024-03-04"}

--- code ---
var bank = readCsv("banco.csv");
var ledger = readCsv("ledger.csv");

var ledgerRefs = {};
for (var i = 0; i < ledger.length; i++) {
  ledgerRefs[ledger[i].reference] = true;
}

var unmatched = bank.filter(function (row) {
  return !ledgerRefs[row.reference];
});

var byDay = {};
for (var j = 0; j < unmatched.length; j++) {
  var row = unmatched[j];
  var date = row.date;
  var amount = parseFloat(row.amount);
  if (!byDay[date]) {
    byDay[date] = { count: 0, total: 0 };
  }
  byDay[date].count += 1;
  byDay[date].total += amount;
}

print("Transacciones del banco sin contraparte en el ledger, agrupadas por día:");
for (var day in byDay) {
  print(day + " - " + byDay[day].count + " transacciones - Total: " + byDay[day].total.toFixed(2));
}
--- output ---
Transacciones del banco sin contraparte en el ledger, agrupadas por día:
2024-03-01 - 1 transacciones - Total: 500.00
2024-03-03 - 1 transacciones - Total: 275.00
2024-03-04 - 1 transacciones - Total: 95.75

**Transacciones del banco sin contraparte en el ledger, agrupadas por día:**

| Fecha | Cantidad | Monto Total |
|-------|----------|-------------|
| 2024-03-01 | 1 | 500.00 |
| 2024-03-03 | 1 | 275.00 |
| 2024-03-04 | 1 | 95.75 |

**Total general:** 3 transacciones por **870.75**

Las referencias sin coincidencia son: `TXN005`, `TXN012` y `TXN018`.
```

El modelo encontró exactamente las 3 transacciones sembradas, con los montos
y fechas correctos, en 5 turnos de código (explorar archivos → inspeccionar
columnas → inspeccionar filas → calcular el join+filter → agrupar por día).
Nota: el modelo usado (`nvidia/nemotron-3-ultra-550b-a55b:free`) es gratuito
y ocasionalmente devuelve una respuesta vacía bajo carga — es una falla
transitoria del proveedor, no del agente; un reintento simple es suficiente.
