package data

// Store owns our own config.db. It is deliberately independent: no first-run
// copy from, and no mirroring back to, the original PortProxyGUI database.
// The file still uses the exact same schema, so a config.db can be exchanged
// manually through Import / Export.
type Store struct {
	own *DB
}

func OpenStore(ownPath string) (*Store, error) {
	own, err := Open(ownPath)
	if err != nil {
		return nil, err
	}
	return &Store{own: own}, nil
}

func (s *Store) Close() error { return s.own.Close() }

func (s *Store) DB() *DB { return s.own }

func (s *Store) Path() string { return s.own.Path() }
