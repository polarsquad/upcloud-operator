// Command cleanup removes every UpCloud resource the operator labelled
// managed-by=uck, plus detached floating IPs in the sample zone. The real-API e2e
// workflow runs it in an always() step after the suite, so a suite killed
// mid-teardown (timeout alarm, cancelled run) cannot leak paid resources. It
// keeps no state from the suite: it selects by label alone, which is safe
// because the test account is dedicated to this suite and the workflow
// serialises runs.
//
// A cancelled run's leftovers are now removed by the next dispatch's preflight
// check, so this tool is a secondary safety net. It exits non-zero when nothing
// was moving by the deadline: no delete was accepted on the last pass, or a
// list failed. Managed databases and object storages delete asynchronously on
// the UpCloud side (a single MOS delete has run past 20 minutes), so a delete
// accepted on the last pass is still converging when the deadline fires; the
// step then reports the leftovers as in flight and exits zero, and the next
// dispatch re-sweeps them.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/test/e2e-upcloud/leftovers"
)

const (
	deadline  = 15 * time.Minute
	pollEvery = 20 * time.Second
)

func main() {
	svc, err := upcloudapi.NewServiceFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	_, last := leftovers.Drain(ctx, svc, os.Stdout, os.Stderr, pollEvery)

	if last.Left() == 0 {
		fmt.Println("cleanup: nothing left")
		return
	}

	// A delete accepted on the last pass is still running on the
	// UpCloud side (managed databases and object storages delete
	// asynchronously), so the account is converging: report it and
	// let the next dispatch's sweep pick up anything that remains.
	if last.Deleted > 0 && last.ListFailures == 0 {
		fmt.Fprintf(os.Stderr, "cleanup: %d resource(s) deleting on the "+
			"UpCloud side at the deadline; re-run the sweep if the "+
			"next dispatch sees leftovers\n", last.Left())
		return
	}
	fmt.Fprintf(os.Stderr, "cleanup: %d resource(s) still present after %s\n", last.Left(), deadline)
	os.Exit(1)
}
