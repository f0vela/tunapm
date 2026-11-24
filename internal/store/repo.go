package store

// Repo es el contrato de datos que usan la TUI, el API y la web app.
// Lo implementan *Store (SQLite local) y el cliente HTTP (internal/client).
type Repo interface {
	CreateClient(c *Client) error
	UpdateClient(c *Client) error
	DeleteClient(id int64) error
	GetClient(id int64) (Client, error)
	ListClients() ([]Client, error)

	CreateProject(p *Project) error
	UpdateProject(p *Project) error
	DeleteProject(id int64) error
	GetProject(id int64) (Project, error)
	ListProjects() ([]Project, error)
	ProjectsByClient(clientID int64) ([]Project, error)
	CountProjectsByClient(clientID int64) (int, error)

	CreateTask(t *Task) error
	UpdateTask(t *Task) error
	DeleteTask(id int64) error
	GetTask(id int64) (Task, error)
	ListTasks(f TaskFilter) ([]Task, error)
	TaskProgress() (map[int64]TaskProgress, error)

	CreatePayment(p *Payment) error
	UpdatePayment(p *Payment) error
	MarkPaymentPaid(id int64, date string) error
	DeletePayment(id int64) error
	GetPayment(id int64) (Payment, error)
	ListPayments() ([]Payment, error)
	PaymentsByProject(projectID int64) ([]Payment, error)
	ProjectTotals(projectID int64) (ProjectTotals, error)
	AllProjectTotals() (map[int64]ProjectTotals, error)

	CreateNote(n *Note) error
	UpdateNote(n *Note) error
	DeleteNote(id int64) error
	GetNote(id int64) (Note, error)
	ListNotes(f NoteFilter) ([]Note, error)
	NoteTags() ([]TagCount, error)

	Snapshot() (Snapshot, error)
}

var _ Repo = (*Store)(nil)
