package interview

import "testing"

func TestStopWorker_DrainsAndCanBeCalledTwice(t *testing.T) {
	svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	svc.StartWorker()
	svc.StopWorker()
	svc.StopWorker()
}
