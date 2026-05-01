package transcript

import "errors"

// ErrTranscriptUnavailable is returned by Transcribe implementations when the
// provider confirms no transcript exists for the video (HTTP 404). Callers
// should use errors.Is to detect this case and handle it without treating the
// video as a pipeline failure.
var ErrTranscriptUnavailable = errors.New("transcript unavailable")
