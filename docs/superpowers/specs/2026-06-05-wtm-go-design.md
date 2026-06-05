# WTM Go - Diseño

**Fecha:** 2026-06-05
**Objetivo:** Reescribir wtm (worktree manager TUI) de Python a Go para producir un binario único distribuible sin dependencia de Python.

---

## Contexto

wtm es un picker TUI interactivo para gestionar git worktrees. La versión Python (~1400 líneas) usa `termios` raw directamente. La versión Go usará el stack charmbracelet (bubbletea + lipgloss + bubbles) para obtener resize automático, popups, hotkeys y links de terminal, manteniendo paridad funcional con mejoras de UX menores.

---

## Estructura del proyecto

Los archivos Python se eliminan. El repo queda:

```
wtm/
  go.mod                   module wtm
  go.sum
  main.go                  entry point: args, tabla o picker
  Makefile                 build → bin/wtm

  internal/
    git/
      parse.go             parsear `git worktree list --porcelain`
      load.go              orquestar git queries → []WorktreeEntry
      pr.go                fetch PRs via `gh` CLI
    tui/
      model.go             Model, Init, Update, View (bubbletea)
      create.go            sub-model creación de worktree
      render.go            columnas, ANSI helpers, lipgloss
      styles.go            constantes de estilo lipgloss
      keys.go              key bindings
    actions/
      run.go               pull, push, rebase, merge, delete, fetch, pruneall → tea.Cmd

  bin/wtm                  binario compilado (make build lo pone aquí)
  wtm.plugin.zsh           sin cambios
  WTM.md                   actualizar sección de instalación
  .wtm-config.yaml         sin cambios
```

**Dependencias (`go.mod`):**
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles`

**Makefile:**
```makefile
build:
    go build -o bin/wtm .
    chmod +x bin/wtm
```

`bin/wtm` pasa de ser un script zsh a ser el binario compilado. El plugin zsh no cambia nada.

---

## Tipos de datos (`internal/git/`)

```go
type MainState string
const (
    MainStateIsMain     MainState = "is_main"
    MainStateIntegrated MainState = "integrated"
    MainStateOrphan     MainState = "orphan"
    MainStateSameCommit MainState = "same_commit"
    MainStateEmpty      MainState = "empty"
    MainStateNone       MainState = ""
)

type CommitInfo struct {
    Timestamp int64
    Message   string
    SHA       string
    ShortSHA  string
}

type WorkingTree struct {
    Staged    bool
    Modified  bool
    Untracked bool
    Deleted   bool
    Renamed   bool
}

type RemoteInfo struct {
    Name   string
    Branch string
    Ahead  int
    Behind int
}

type MainInfo struct {
    Ahead  int
    Behind int
}

type WorktreeEntry struct {
    Branch      string
    Path        string
    Head        string
    Detached    bool
    Prunable    bool
    Locked      bool
    Commit      CommitInfo
    WorkingTree WorkingTree
    MainState   MainState
    Main        MainInfo
    Remote      *RemoteInfo  // nil si no tiene upstream
    IsCurrent   bool
    IsMain      bool
}

type PRInfo struct {
    Number int
    State  string  // "OPEN", "MERGED", "CLOSED"
    Draft  bool
}
```

`git.Load() ([]WorktreeEntry, error)` orquesta todos los queries (equivalente a `_load_data()` en Python):
1. `git worktree list --porcelain` → parse → slice de raw entries
2. Para cada entry: commit info, working tree status, upstream, merge-base vs main
3. Determina `IsCurrent` (worktree que contiene `os.Getwd()`)

`git.FetchPRMap(root string) map[string]PRInfo` llama a `gh pr list --json ...` (equivalente a `fetch_pr_map()`).

---

## Arquitectura TUI (`internal/tui/`)

### Model

```go
type mode int
const (
    modeNormal mode = iota
    modeCreating       // subpantalla de creación activa
    modeActionResult   // overlay con log de operación
)

type Model struct {
    worktrees     []git.WorktreeEntry
    selected      int
    scroll        int
    width, height int

    prMap     map[string]git.PRInfo
    prLoading bool
    spinner   spinner.Model

    mode mode

    // overlay de resultado
    actionLog  []string
    actionDone bool  // true = esperando keypress para cerrar

    // sub-model de creación
    create *CreateModel

    cdfile     string
    root       string
    selectPath string
    colWidths  ColWidths
}
```

### Mensajes

```go
type prLoadedMsg    struct{ m map[string]git.PRInfo }
type worktreesMsg   struct{ entries []git.WorktreeEntry }
type actionDoneMsg  struct{ log []string; selectPath string; cdPath string }
type refreshDoneMsg struct{}
type refreshTickMsg struct{}
```

### Init

Lanza en paralelo:
- `worktreesCmd` (reload inicial, aunque ya tenemos los datos del arranque)
- `loadPRsCmd(root)` (goroutine, devuelve `prLoadedMsg`)
- `refreshTickCmd()` (tick de 60s)
- `spinner.Tick` (mientras `prLoading`)

### Update - flujo

```
KeyMsg → según mode:
  modeNormal      → navegación / acción / abrir create
  modeCreating    → delegar a CreateModel.Update
  modeActionResult→ cualquier tecla cierra overlay

prLoadedMsg    → guardar prMap, prLoading=false
worktreesMsg   → guardar worktrees, recalcular colWidths
actionDoneMsg  → si cdPath != "": escribir cdfile + tea.Quit
               → si solo log: modeActionResult + actionLog
refreshTickMsg → lanzar fetchCmd + encolar siguiente tick
refreshDoneMsg → reload worktrees
WindowSizeMsg  → actualizar width/height, scroll
```

### Lazygit

```go
tea.ExecProcess(exec.Command("lazygit"), func(err error) tea.Msg {
    return actionDoneMsg{selectPath: currentPath}
})
```

Cede el terminal a lazygit limpiamente. Al salir, bubbletea restaura el TUI y preselecciona el worktree anterior.

### Overlay de resultado

Las operaciones con output (pull, push, rebase, merge, fetch, delete, pruneall) corren como `tea.Cmd`, recogen `CombinedOutput()` y devuelven `actionDoneMsg{log: ...}`. El overlay muestra las líneas del log con estilo DIM y una línea "Press any key" al final. Mejora UX respecto al Python (el log aparece dentro del TUI, no en el terminal crudo).

### Refresh automático

Tick de 60s → `git fetch --all --quiet` en background → `refreshDoneMsg` → reload worktrees. No hay daemon goroutine; se usan `tea.Cmd` encadenados.

---

## Subpantalla de creación (`internal/tui/create.go`)

Máquina de estados embebida como sub-model de `Model`.

### Estados

```go
type createStep int
const (
    stepBranches      createStep = iota
    stepAction                   // "Use directly" / "New branch"
    stepNewBranchName
    stepWtName
    stepExecuting                // spinner mientras git worktree add
    stepDone                     // resultado antes de volver
)
```

### CreateModel

```go
type CreateModel struct {
    step          createStep
    branches      []BranchInfo  // {Name, IsRemote, ExistingWT string}
    filter        string
    brSelected    int
    brScroll      int

    chosenBranch   string
    chosenIsRemote bool
    chosenHasWT    bool
    flow           string   // "direct" | "new_branch"
    newBranchName  string
    defaultWtName  string

    textInput  textinput.Model  // bubbles - reutilizado para branch name y wt name
    actionSel  int              // 0=Use directly, 1=New branch

    executing bool
    execLog   []string
    execErr   bool

    width, height int
    root          string
    data          []git.WorktreeEntry
}
```

### Mejoras UX respecto al Python

- `stepExecuting` muestra un spinner mientras `git worktree add` corre (el Python bloqueaba la pantalla)
- `textinput.Model` de bubbles maneja cursor, backspace y paste correctamente (el Python tenía implementación manual)
- El filtro de ramas usa el mismo `textInput`, sin lógica especial de lectura de chars

### Hooks de creación

El parser de `.wtm-config.yaml` se reimplementa en Go sin dependencia de un paquete YAML externo (misma lógica line-by-line que el Python). Los hooks se ejecutan secuencialmente tras `git worktree add` y su output aparece en `execLog` dentro de `stepDone`.

### Delegación desde Model principal

- `modeCreating`: cada `KeyMsg` va a `createModel.Update()`
- Al terminar, `CreateModel` devuelve `tea.Cmd` que emite `actionDoneMsg{selectPath: createdPath}` o nada si se canceló
- `Model` vuelve a `modeNormal`, hace reload y preselecciona el path creado

---

## Acciones (`internal/actions/run.go`)

Cada acción es una función que devuelve `tea.Cmd`:

```go
func PullCmd(path string) tea.Cmd
func PushCmd(path, branch string) tea.Cmd
func RebaseCmd(path, mainBranch string) tea.Cmd
func MergeCmd(path, mainBranch string) tea.Cmd
func FetchCmd(root string) tea.Cmd
func DeleteCmd(path string, force bool) tea.Cmd
func PruneAllCmd(root string, entries []git.WorktreeEntry) tea.Cmd
func CDCmd(path, cdfile string) tea.Cmd  // escribe cdfile y emite tea.Quit
```

Todas usan `os/exec`. `RebaseCmd` y `MergeCmd` hacen fetch + actualización del branch local antes de rebase/merge, igual que el Python. `PruneAllCmd` replica la lógica de Python: fetch --prune, worktree prune, luego borra worktrees integrados con más de 1 día y sin cambios.

---

## Entry point (`main.go`)

```go
func main() {
    cdfile    // argv[1] si no empieza por "-", o ""
    pickMode  // cdfile != "" || flag --pick
    selectPath // flag --select=<path>

    entries, err := git.Load()
    if err != nil { os.Exit(1) }

    if !pickMode {
        printTable(entries)  // mismo modo tabla que el Python
        return
    }

    m := tui.NewModel(entries, cdfile, selectPath)
    p := tea.NewProgram(m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

---

## Protocolo de integración con zsh (sin cambios)

`wtm.plugin.zsh` define:
```zsh
function wtm() {
  local _tmp; _tmp=$(mktemp)
  "$_WTM_DIR/bin/wtm" "$_tmp"
  local _path; _path=$(<"$_tmp" 2>/dev/null)
  rm -f "$_tmp"
  [[ -n "$_path" ]] && cd "$_path"
}
```

El binario Go escribe el path en `cdfile` (argv[1]) y sale. La función zsh lee el path y hace `cd`. Sin cambios en el protocolo.

---

## Lo que desaparece

- `wtm/` (directorio Python entero)
- `wtm.py`
- Dependencia de `/opt/homebrew/bin/python3`
- `bin/wtm` como script zsh (pasa a ser el binario compilado)

## Lo que no cambia

- `wtm.plugin.zsh`
- Formato de `.wtm-config.yaml`
- Protocolo cdfile
- Teclas y comportamiento funcional
- `WTM.md` (solo se actualiza la sección de requisitos e instalación)
