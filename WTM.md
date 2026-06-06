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
| `D` | prune all: fetch --prune + worktree prune + borra worktrees MERGED limpios con >1 dia |
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
| `Changes` | `S`=staged · `M`=modificado · `?`=untracked · `D`=eliminado · `-`=limpio |
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
| `empty` | sin commits todavia |
| `←N` (verde) | N commits por delante de main |
| `→N` (azul) | N commits por detras de main (puedes hacer r o u) |
| `←N →M` | diverged: N propios, M de main sin integrar |

## Configuracion (.wtm-config.yaml)

wtm busca un fichero `.wtm-config.yaml` en la raiz del repositorio. Si existe, carga la configuracion al ejecutar operaciones.

```yaml
# un solo hook
hooks:
  create-worktree: /ruta/al/script.sh

# varios hooks (se ejecutan en orden)
hooks:
  create-worktree:
    - /ruta/al/primero.sh
    - /ruta/al/segundo.sh
```

### hooks.create-worktree

Si esta definido, wtm ejecuta ese script (o scripts) justo despues de crear un worktree nuevo (tecla `C`). Cada script recibe dos argumentos posicionales.

Si un fichero no existe, se muestra un aviso y se continua con el siguiente sin fallar.

```
<script> <wt_path> <wt_name>
```

| Argumento | Valor |
|-----------|-------|
| `$1` | path absoluto del worktree recien creado |
| `$2` | nombre del worktree (el directorio, sin la ruta completa) |

La salida del script (stdout y stderr) se muestra en pantalla antes de volver al picker.

Ejemplo de script:

```sh
#!/bin/sh
# $1 = path del worktree, $2 = nombre
cd "$1"
cp /ruta/a/.env .env
echo "Entorno copiado"
```

## Por que debe ser una funcion de shell, no un script

`cd` solo funciona en el proceso actual de la shell. Un script subprocess no puede cambiar el directorio del padre. Por eso `wtm` es una `function` en `.zshrc`: el script escribe el path final, la funcion lo lee y hace el `cd`.

## Componentes

### `bin/wtm`
Binario compilado en Go. Gestiona el TUI con bubbletea/lipgloss, todas las acciones (lazygit, pull, push, rebase, merge, delete, prune, fetch, crear worktree) y la tabla sin picker. Acepta `--select=<path>` para pre-seleccionar un worktree al arrancar (usado tras crear uno nuevo).

Tecla `C` abre la subpantalla de creacion inline: lista todas las ramas locales y remotas con filtrado por texto, menu de accion con navegacion horizontal, inputs de texto para nombre de rama y nombre de worktree, y ejecucion directa de `git worktree add`.

### `wtm.plugin.zsh`
Define la funcion shell `wtm()`: llama a `bin/wtm` con el path del fichero temporal como `$1`. La funcion lee ese fichero y hace el `cd` (unica operacion que no puede hacer un subproceso).

## Requisitos

- **`git`** en PATH — requerido
- **`lazygit`** en PATH — opcional, solo para tecla `l`
- **`gh`** en PATH — opcional, solo para columna PR

## Instalacion

Compilar el binario:

```bash
make build
```

Añadir en `~/.zshrc`:

```zsh
source /path/to/wtm/wtm.plugin.zsh
```
