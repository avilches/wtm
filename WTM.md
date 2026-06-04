# wtm — navegador interactivo de worktrees

Picker TUI para listar, navegar y operar sobre worktrees git.

## Uso

```
wtm
```

## Teclas

### Navegacion (siempre disponibles)

| Tecla | Accion |
|-------|--------|
| `↑` / `k` | subir |
| `↓` / `j` | bajar |
| `g` | ir al primero |
| `G` | ir al ultimo |
| `Enter` | cd al worktree seleccionado |
| `C` | crear worktree (subpantalla inline) |
| `l` | cd + abrir lazygit (vuelve al picker al salir) |
| `f` | fetch --all desde el root del repo |
| `D` | prune all: llama a `wt step prune` |
| `q` / `Esc` / `Ctrl+C` | salir sin hacer nada |

### Operaciones condicionales (solo si aplican al worktree seleccionado)

| Tecla | Condicion | Accion |
|-------|-----------|--------|
| `p` | `remote.behind > 0` | pull desde el remote tracking branch |
| `P` | `remote.ahead > 0` o sin remote | push (con `-u origin <branch>` si no hay remote) |
| `r` | `main.behind > 0` | rebase desde main (reescribe historial) |
| `u` | `main.behind > 0` | merge desde main (preserva historial) |
| `d` | no es main | borrar worktree (pide confirmacion, excepto si es orphan) |

## Columnas

| Columna | Significado |
|---------|-------------|
| `@` | worktree actual (donde estas) |
| `Branch` | nombre de la rama. Verde si es la actual, gris si es candidata a borrar |
| `Flags` | `S`=staged · `M`=modificado · `?`=untracked · `D`=eliminado · `-`=limpio |
| `State` | estado respecto a main (ver abajo) |
| `Remote` | `up to date` · `↓N`=commits para pullear · `↑N`=commits para pushear · `no-remote` |
| `Age` | tiempo desde el ultimo commit |
| `Commit` | mensaje del ultimo commit |

### Valores de State

| Valor | Significado |
|-------|-------------|
| `main` | este ES el worktree principal |
| `MERGED` | ya integrado en main, se puede borrar |
| `ORPHAN` | la rama remota fue borrada, se puede borrar |
| `=` | mismo commit que main |
| `C` (amarillo) | tiene cambios que conflictuan con main |
| `empty` | sin commits todavia |
| `←N` (verde) | N commits por delante de main |
| `→N` (azul) | N commits por detras de main (puedes hacer r o u) |
| `←N →M` | diverged: N propios, M de main sin integrar |

## Por que debe ser una funcion de shell, no un script

`cd` solo funciona en el proceso actual de la shell. Un script subprocess no puede cambiar el directorio del padre. Por eso `wtm` es una `function` en `.zshrc`: el script escribe el path final, la funcion lo lee y hace el `cd`.

## Componentes

### `wtm.py`
App Python completa. Gestiona el TUI con `termios`, el bucle de acciones (lazygit, pull, push, rebase, merge, delete, prune, fetch, crear worktree) y la tabla sin picker. Acepta `--select=<path>` para pre-seleccionar un worktree al arrancar (usado tras crear uno nuevo).

Tecla `C` abre la subpantalla de creacion inline: lista todas las ramas locales y remotas con filtrado por texto, menu de accion con navegacion horizontal, inputs de texto para nombre de rama y nombre de worktree, y ejecucion directa de `git worktree add`.

Para lazygit: `signal.SIG_IGN` en Python + `preexec_fn` que restaura `SIG_DFL` antes de `exec lazygit`, de modo que Ctrl+C sale de lazygit pero no del picker.

### `bin/wtm`
Wrapper zsh de 3 lineas: llama a `wtm.py` con el path del fichero temporal como `$1`. La funcion en `.zshrc` lee ese fichero y hace el `cd` (unica operacion que no puede hacer un subproceso).

## Requisitos

- **`/opt/homebrew/bin/wt`** — binario de worktrunk
- **Python 3** en `/opt/homebrew/bin/python3`
- **`lazygit`** — opcional, solo para tecla `l`
- **`eval "$(/opt/homebrew/bin/brew shellenv)"`** en `.zshrc` para que PATH este disponible en subprocesos

## Instalacion

Añadir en `~/.zshrc`:

```zsh
function wtm() {
  local _tmp; _tmp=$(mktemp)
  /Users/avilches/Work/Proy/Other/wtm/bin/wtm "$_tmp"
  local _path; _path=$(<"$_tmp" 2>/dev/null)
  rm -f "$_tmp"
  [[ -n "$_path" ]] && cd "$_path"
}
```

O añadir `bin/` al PATH y usar la funcion con `wtm "$_tmp"`.
