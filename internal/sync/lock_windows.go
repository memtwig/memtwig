//go:build windows

package sync

func acquireRepoLock(root string) (func(), error) {
	return func() {}, nil
}
