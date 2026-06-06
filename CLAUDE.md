# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Worktree manager

TUI interactivo para navegar y operar worktrees git. Escrito en Go con bubbletea/lipgloss.

## Compilar y ejecutar

```bash
# Compilar
make build           # produce bin/wtm

# Modo tabla (sin TUI, imprime la lista y sale)
./bin/wtm

# Modo picker interactivo (TUI completo)
./bin/wtm /tmp/wtm_test

# Preseleccionar un worktree al abrir
./bin/wtm /tmp/wtm_test --select=/path/to/wt
```

No hay tests automatizados. La verificación es siempre manual ejecutando el binario.

## Estructura

```
main.go                     — entry point (modo tabla y picker)
internal/
  git/
    types.go                — tipos: WorktreeEntry, PRInfo, CommitInfo
    parse.go                — parser de `git worktree list --porcelain`
    load.go                 — Load(), GetMainBranch(), buildEntry()
    pr.go                   — FetchPRMap(), RepoURL() via gh CLI
  tui/
    styles.go               — variables lipgloss
    render.go               — ColWidths, BuildLine, MakeHeader, DetailLine
    model.go                — Model bubbletea principal + handleKey + predicados
    create.go               — CreateModel: subpantalla de creación de worktrees
  actions/
    run.go                  — CDCmd, PullCmd, PushCmd, RebaseCmd, MergeCmd, FetchCmd, DeleteCmd, PruneAllCmd
Makefile                    — `make build`
wtm.plugin.zsh              — plugin zsh que define la función wtm()
WTM.md                      — documentación de usuario
```

## Arquitectura

### Flujo de datos

1. `git.Load()` ejecuta `git worktree list --porcelain` y construye `[]WorktreeEntry` con datos de rama, estado, remote, commits y PR
2. `main.go` arranca bubbletea con `NewModel()` o imprime la tabla si no hay cdfile
3. El loop de bubbletea maneja mensajes: teclas, PR cargado, ticks de refresco, resultados de acciones
4. Las acciones son `tea.Cmd` que corren en goroutines y devuelven `ActionDoneMsg`
5. `CreateModel` es un sub-modelo independiente que maneja la subpantalla de creación

### Modos del modelo bubbletea (model.go)

| Modo | Descripción |
| ---- | ----------- |
| `modeNormal` | vista principal con tabla |
| `modeCreating` | subpantalla de creación (delega a `CreateModel`) |
| `modeActionResult` | overlay con el log de la última acción |
| `modeConfirm` | confirmación de borrado |

### Columna PR (carga asíncrona)

Se carga como `tea.Cmd` al arrancar. Mientras carga, el spinner anima con ticks de 100ms (`spinTickMsg`). Al completar devuelve `prLoadedMsg`. Refresco automático de worktrees cada 60 segundos con `refreshTickMsg`.

### Subpantalla de creación (tecla `C`)

`CreateModel` es una máquina de estados:

```
stBranches -> stAction (si la rama no tiene worktree)
           -> stNewBranchName (si ya tiene worktree o elige "New branch")
stNewBranchName -> stWTName
stAction -> stWTName (si elige "Use directly")
stWTName -> execute
execute -> createDoneMsg{path} (éxito) | stBranches (fallo, reset)
```

Los worktrees se crean en `.claude/worktrees/<nombre>` dentro del root del repo.

### Hooks (.wtm-config.yaml)

Tras crear un worktree, `readCreateHooks()` parsea `.wtm-config.yaml` en el root del repo y ejecuta los scripts definidos en `hooks.create-worktree`. Cada script recibe `<wt_path> <wt_name>` como argumentos posicionales.

## Por qué es función de shell y no script

`cd` solo funciona en el proceso actual de la shell. `wtm.plugin.zsh` define `wtm()` como función: el binario escribe el path destino en un tmpfile, la función lo lee y hace el `cd`.

## Requisitos del sistema

- `git` en PATH — requerido
- `lazygit` en PATH — opcional, solo para tecla `l`
- `gh` en PATH — opcional, solo para columna PR
