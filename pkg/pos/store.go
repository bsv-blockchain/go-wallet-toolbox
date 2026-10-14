package pos

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// FileStore is for a single service process. Do not share across processes.
// Production multi-instance deployments must provide a transactional Store
// with a merchant-wide lock covering service.Call and wallet acceptance.
type FileStore struct{ Path string }

func (s FileStore) Load(ctx context.Context) (*State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return EmptyState(), nil
	}
	if err != nil {
		return nil, err
	}
	state := EmptyState()
	if err = json.Unmarshal(raw, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (s FileStore) Save(ctx context.Context, state *State) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := marshal(state)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(s.Path+".tmp", s.Path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.Path))
	if err != nil {
		return err
	}
	syncErr := dir.Sync()
	closeErr = dir.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
