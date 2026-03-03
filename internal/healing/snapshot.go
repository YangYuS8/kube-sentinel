package healing

import "fmt"

type Snapshot struct {
	Namespace string
	Name      string
	Revision  string
}

type Snapshotter interface {
	Create(namespace, name string) (Snapshot, error)
	Restore(snapshot Snapshot) error
}

type MemorySnapshotter struct {
	Snapshots []Snapshot
}

func (m *MemorySnapshotter) Create(namespace, name string) (Snapshot, error) {
	s := Snapshot{Namespace: namespace, Name: name, Revision: "current"}
	m.Snapshots = append(m.Snapshots, s)
	return s, nil
}

func (m *MemorySnapshotter) Restore(snapshot Snapshot) error {
	if snapshot.Name == "" {
		return fmt.Errorf("invalid snapshot")
	}
	return nil
}
