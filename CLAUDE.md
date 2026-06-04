# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# wtm — worktree manager

TUI interactivo para navegar y operar worktrees git via `worktrunk` (`wt`).

## Estructura

```
wtm/
  wtm.py            — app completa (TUI + action handlers)
  bin/wtm           — wrapper zsh (exec al Python, pasa temp file)
  wtm.plugin.zsh    — plugin de instalacion (source desde .zshrc)
  WTM.md            — documentacion de usuario
```

## Ejecutar / probar

```bash
# Modo tabla (sin TUI, util para debuggear datos)
/opt/homebrew/bin/python3 wtm.py

# Modo picker interactivo (TUI completo)
/opt/homebrew/bin/python3 wtm.py /tmp/wtm_test

# Inyectar datos de prueba via stdin
echo '[...]' | /opt/homebrew/bin/python3 wtm.py /tmp/wtm_test

# Preseleccionar un worktree al abrir
/opt/homebrew/bin/python3 wtm.py /tmp/wtm_test --select=/path/to/wt
```

No hay tests automatizados. La verificacion es siempre manual.

## Instalacion

```zsh
# En ~/.zshrc:
source /path/to/wtm/wtm.plugin.zsh
```

`wtm.plugin.zsh` define la funcion `wtm()` que crea un tmpfile, invoca `bin/wtm`, lee el path resultante y hace `cd`. El `cd` debe ocurrir en el proceso padre de la shell, por eso es funcion y no script.

## Arquitectura

### Flujo de datos

1. `_load_data()` ejecuta `wt list --format json` y parsea el JSON
2. `run_picker()` renderiza el TUI en `/dev/tty` en modo raw (`termios`)
3. El picker devuelve un sentinel string: `"ACTION:path"` o `None`
4. `_handle_action()` ejecuta la accion y devuelve `(continue_loop, select_path)`
5. Si `continue_loop`, se recarga data con `_load_data()` y se reabre el picker

### Protocolo de sentinels

El picker devuelve uno de estos strings al bucle principal en `main()`:

| Sentinel | Descripcion |
|----------|-------------|
| `CD:<path>` | Escribir path en cdfile y salir |
| `LAZYGIT:<path>` | Escribir path en cdfile, lanzar lazygit, volver al picker |
| `PULL:<path>` | `git pull` en ese worktree |
| `PUSH:<path>` | `git push -u origin <branch>` |
| `REBASE:<path>` | fetch + rebase desde main |
| `MERGE:<path>` | fetch + merge desde main |
| `FETCH:<path>` | `git fetch --all` desde el root del repo |
| `DELETE:<path>` | `git worktree remove` con confirmacion |
| `DELETEORPHAN:<path>` | `git worktree remove --force` sin confirmacion |
| `PRUNEALL:<path>` | `wt step prune` en el root |
| `CREATED:<path>` | Worktree creado; reabrir picker preseleccionando ese path |

### Columna PR (carga asincrona)

La columna PR se carga en un thread daemon (`_load_prs`). Durante la carga, el spinner anima en cada tick de 0.1s via `select` timeout. Cuando el thread termina, setea `_pr_ready` (Event) y el bucle principal re-renderiza. Estado en globals: `pr_map`, `_pr_repo_url`, `_pr_state`.

### Subpantalla de creacion (tecla `C`)

`run_create_picker()` es una maquina de estados inline que reutiliza el mismo fd `/dev/tty` ya abierto en raw mode:

```
branches -> action (si branch sin worktree existente)
         -> new_branch_name (si branch ya tiene worktree, o si elige "New branch")
new_branch_name -> wt_name
action -> wt_name (si elige "Use directly")
wt_name -> execute
execute -> CREATED:<path> (exito) | branches (fallo, reset)
```

Los worktrees se crean en `.claude/worktrees/<nombre>` dentro del root del repo.

### SIGINT y lazygit

Lazygit necesita `SIGINT` para responder a Ctrl+C. El handler: Python ignora SIGINT (`SIG_IGN`) antes de `subprocess.run`, y usa `preexec_fn=_reset_sigint` para que el proceso hijo restaure `SIG_DFL` antes de ejecutarse. Al volver, se restaura el handler original de Python.

## Requisitos del sistema

- `/opt/homebrew/bin/wt` — worktrunk (fuente de datos)
- `/opt/homebrew/bin/python3` — Python 3.9+
- `lazygit` en PATH — opcional, solo para tecla `l`
- `gh` en PATH — opcional, solo para columna PR
