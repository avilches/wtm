# Handoff: WTM rewrite Python a Go

**Fecha:** 2026-06-05
**Rama:** `go`
**Worktree:** `.claude/worktrees/go`

---

## Contexto

WTM es un picker TUI interactivo para gestionar git worktrees. La app Python existente (~1400 líneas, en `wtm/`) se va a reescribir en Go para producir un binario único sin dependencia de Python.

En esta sesión se hizo brainstorming completo, se eligió el stack, se escribió el spec y el plan de implementación. **El siguiente paso es ejecutar el plan.**

---

## Estado actual

El plan de implementación está escrito y commiteado. El código Python sigue intacto (aún no se ha tocado). El worktree `go` existe para aislar el trabajo.

**Archivos creados en esta sesión:**

| Archivo | Descripción |
|---------|-------------|
| `docs/superpowers/specs/2026-06-05-wtm-go-design.md` | Spec completo de diseño |
| `docs/superpowers/plans/2026-06-05-wtm-go.md` | Plan de implementación con código |

**Ningún archivo de código Go ha sido creado todavía.**

---

## Decisiones tomadas

- **Stack TUI:** bubbletea + lipgloss + bubbles/textinput (charmbracelet)
- **Sin otras dependencias:** no bubbletea/spinner (spinner manual con tick), no bubbletea/list (lista propia)
- **Reemplazar Python directamente:** los archivos Python se borran al final
- **Paridad funcional + mejoras UX menores:**
  - Resultado de acciones (pull/push/rebase) aparece dentro del TUI en overlay, no en terminal crudo
  - Subpantalla de creación muestra spinner durante `git worktree add`
  - Confirmación de borrado en overlay (`modeConfirm`), no con termios raw

---

## El plan (resumen de tareas)

Ver plan completo: `docs/superpowers/plans/2026-06-05-wtm-go.md`

| Task | Archivos | Estado |
|------|----------|--------|
| 1 | `go.mod`, `Makefile` | pendiente |
| 2 | `internal/git/types.go` | pendiente |
| 3 | `internal/git/parse.go` | pendiente |
| 4 | `internal/git/load.go` | pendiente |
| 5 | `internal/git/pr.go` | pendiente |
| 6 | `internal/tui/styles.go`, `render.go` | pendiente |
| 7 | `internal/actions/run.go` | pendiente |
| 8 | `internal/tui/model.go` | pendiente |
| 9 | `internal/tui/create.go` | pendiente |
| 10 | `main.go` | pendiente |
| 11 | Cleanup Python, update docs | pendiente |

---

## Puntos de atención para la implementación

1. **Imports de `os/exec` en model.go:** usado para `exec.LookPath("lazygit")` y `fetchAndReloadCmd`.
2. **`min`/`max` en Go 1.21:** son builtins. Las funciones custom en `render.go` las sobreescriben pero compilan igual. Opcional eliminarlas y usar builtins.
3. **Delegación de mensajes al sub-model create:** los `tea.KeyMsg` van via `handleKey`, los demás (como `createExecDoneMsg`) van via el bloque de delegación al final de `Update`.
4. **`PruneAllCmd(root string)`** - solo toma root, llama a `git.Load()` internamente para datos frescos.
5. **OSC 8 links** generados con ANSI raw (`\033]8;;URL\033\\`), no con lipgloss.
6. **`bin/wtm`** pasa de script zsh a binario compilado. `wtm.plugin.zsh` no cambia.

---

## Skills sugeridas

- `superpowers:subagent-driven-development` - para ejecutar el plan task a task con subagentes (opción recomendada)
- `superpowers:executing-plans` - para ejecución inline con checkpoints
- `superpowers:verification-before-completion` - antes de dar el task 11 por terminado

---

## Cómo continuar

```bash
# El worktree ya existe
cd /Users/avilches/Work/Proy/Other/wtm/.claude/worktrees/go

# Leer el plan y empezar a ejecutarlo
# Task 1: go.mod + Makefile
```

Referencia al plan: `docs/superpowers/plans/2026-06-05-wtm-go.md`
