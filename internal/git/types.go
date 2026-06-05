package git

// MainState describe el estado del worktree respecto a main.
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
	Remote      *RemoteInfo // nil si no tiene upstream
	IsCurrent   bool
	IsMain      bool
}

type PRInfo struct {
	Number int
	State  string // "OPEN", "MERGED", "CLOSED"
	Draft  bool
}
