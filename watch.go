package gols

import (
	"context"

	"github.com/drgo/core/watcher"
)

func watch(ctx context.Context) error {
	// channel to receive os nofitications
	events := make(chan watcher.Event)
	////channel to communicate with workers
	//resCh := make(chan *task)
	////A counting semaphore to limit number of concurrently open files
	//tokens := NewCountingSemaphore(job.RosewoodSettings.MaxConcurrentWorkers)
	//go func() { //launch a routine to call htmlRunner on each received notification
	//	for {
	//		select {
	//	case event := <-events:
	//			if !event.IsWrite() || filepath.Ext(event.Name) != ".txt" {
	//				continue
	//			}
	//			tokens.Reserve(1)                     //reserve a worker
	//			go htmlRunner(event.Name, job, resCh) //launch a worker
	//		case res := <-resCh:
	//			tokens.Free(1) //release a reserved worker
	//			if res.err != nil {
	//				job.UI.Warn("failed to process: " + res.inputFileName + ":" +
	//					res.err.Error())
	//			} //errors do not terminate watching
	//			if err := genIndexFile(filepath.Join(job.RunOptions.WorkDirName, "index.html"), nil); err != nil {
	//				job.UI.Warn("failed to regenerate index.html: " + err.Error())
	//			}
	//		case <-ctx.Done(): //context cancelled
	//			return
	//		}
	//	}
	//}()

	// start watcher
	err := watcher.Watch(ctx, "", events)
	return err
}
