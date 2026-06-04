# wtm — worktree manager

Proyecto Python independiente. TUI interactivo para navegar y operar worktrees git via `worktrunk` (`wt`).

## Estructura

```
wtm/
  wtm.py          — app completa (TUI + action handlers)
  bin/
    wtm           — wrapper zsh (solo cd, pasa temp file al Python)
  WTM.md          — documentacion de usuario (teclas, columnas, protocolo)
  CLAUDE.md       — este archivo
```

## Uso

El comando `wtm` es una funcion de shell en `.zshrc` que llama a `bin/wtm` y hace `cd` con el path que devuelve. Ver `WTM.md` para documentacion completa.

## Desarrollo

`wtm.py` es un script Python stdlib puro (sin dependencias externas). Todo el TUI usa `termios`/`tty`. Las acciones (lazygit, git rebase, git merge, push, pull, delete worktree, etc.) se ejecutan con `subprocess.run()` directamente en el terminal.

### Flujo de datos

1. `wtm.py` lee JSON de `wt list --format json` (via `/opt/homebrew/bin/wt`)
2. Renderiza TUI en `/dev/tty`
3. Al seleccionar una accion, sale del TUI, ejecuta la accion con output visible
4. Para CD: escribe el path en el fichero temporal (`sys.argv[1]`), termina
5. La funcion shell en `.zshrc` lee el fichero y hace `cd`

### Requisitos

- `/opt/homebrew/bin/wt` (worktrunk)
- `/opt/homebrew/bin/python3` (Python 3.9+)
- `lazygit` en PATH (opcional, solo para tecla `l`)
