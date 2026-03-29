package main

import (
	"context"
	"fmt"
	"log/slog"
)

func (d *Daemon) getUserPass(ctx context.Context, username string) (string, error) {
	d.userTokensLock.RLock()
	pass, ok := d.userTokens[username]
	d.userTokensLock.RUnlock()
	if ok {
		return pass, nil
	}
	pp, err := d.mailcowClient.GetAppPasswords(ctx, username)
	if err != nil {
		return "", err
	}
	oldIDs := make([]int, 0)
	for _, p := range pp {
		if p.Name == ConstUsertokenName {
			oldIDs = append(oldIDs, p.ID)
		}
	}
	if len(oldIDs) > 0 {
		slog.DebugContext(ctx, "removing old app passwords", "user", username, "count", len(oldIDs))
	}
	if err := d.mailcowClient.DeleteAppPasswords(ctx, oldIDs); err != nil {
		return "", fmt.Errorf("error deleting app passwords: %w", err)
	}
	pass, err = randomPassword(32)
	if err != nil {
		return "", fmt.Errorf("error generating password: %w", err)
	}
	if err := d.mailcowClient.CreateAppPassword(ctx, username, ConstUsertokenName, pass, "dav_access"); err != nil {
		return "", err
	}
	slog.InfoContext(ctx, "created new app password", "user", username)
	d.userTokensLock.Lock()
	d.userTokens[username] = pass
	d.stateUnsaved = true
	d.userTokensLock.Unlock()
	return pass, nil
}
