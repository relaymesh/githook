package worker

import "context"

func (w *Worker) updateEventLogStatus(ctx context.Context, logID, status string, err error) {
	if logID == "" {
		return
	}
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	client := &EventLogsClient{BaseURL: installationsBaseURL()}
	if updateErr := client.UpdateStatus(ctx, logID, status, errorMessage); updateErr != nil && w != nil {
		if w.logger != nil {
			w.logger.Printf("event log update failed: %v", updateErr)
		}
	}
}
