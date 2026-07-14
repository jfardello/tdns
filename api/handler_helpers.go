package api

import (
	"context"
	"errors"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
)

func (api *v1) overrideStore(ctx context.Context) (*overrides.Store, error) {
	return overrides.Open(ctx, config.GetRunningConfig().Database.File)
}

func replaceOverrideValues(ctx context.Context, store *overrides.Store, kind overrides.Kind, values []string) error {
	if err := store.DeleteByKind(ctx, kind); err != nil {
		return err
	}
	for _, each := range values {
		if err := store.Upsert(ctx, kind, overrides.OverrideUpsert, each, ""); err != nil {
			return err
		}
	}
	return nil
}

func replaceOverrideHosts(ctx context.Context, store *overrides.Store, kind overrides.Kind, hosts map[string]string) error {
	if err := store.DeleteByKind(ctx, kind); err != nil {
		return err
	}
	for domain, address := range hosts {
		if err := store.Upsert(ctx, kind, overrides.OverrideUpsert, domain, address); err != nil {
			return err
		}
	}
	return nil
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}

func middlewareCloneHosts(hosts map[string]string) map[string]string {
	cloned := make(map[string]string, len(hosts))
	for domain, address := range hosts {
		cloned[domain] = address
	}
	return cloned
}

func actionToBool(action string) (bool, error) {
	switch action {
	case "start":
		return true, nil
	case "stop":
		return false, nil

	}
	return false, errors.New("Invalid parameter.")
}
