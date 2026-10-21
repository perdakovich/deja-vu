package inspect

import "github.com/vshulcz/deja-vu/internal/sources"

// The inspection layer consumes the same native reader as search ingestion.
// These aliases keep source-specific parsing in internal/sources while the
// variables leave a narrow seam for hermetic projection tests.
type DeepSeekHeader = sources.DeepSeekHeader
type DeepSeekEvent = sources.DeepSeekEvent
type DeepSeekTrace = sources.DeepSeekTrace

var readDeepSeekTrace = sources.ReadDeepSeekTrace
var canonicalDeepSeekLog = sources.CanonicalDeepSeekLog
var findDeepSeekSessionLog = sources.FindDeepSeekSessionLog
