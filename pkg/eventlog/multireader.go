package eventlog

import "github.com/gomods/athens/pkg/storage"

type multiReader struct {
	logs    []PointedLog
	checker storage.Checker
}

// PointedLog is collection of event logs with specified starting pointers used by ReadFrom function.
// TODO: please come up with better name, this one is horrible
type PointedLog struct {
	Log   Eventlog
	Index int64
}

// NewMultiReader creates composite reader of specified readers.
// Order of readers matters in a way how Events are deduplicated.
// Initial state:
// - InMemory [A, B] - as im.A, im.B
// R1: [C,D,E] - as r1.C...
// R2: [A,D,F]
// R3: [B, G]
// result [im.A, im.B, r1.C, r1.D, r1.E, r2.F, r3.G]
// r2.A, r2.D, r3.B - skipped due to deduplication checks
func NewMultiReader(ch storage.Checker, ll ...Eventlog) Reader {
	logs := make([]PointedLog, 0, len(ll))
	for _, l := range ll {
		// init to -1, not 0, 0 might mean first item and as this is excluding pointer we might lose it
		logs = append(logs, PointedLog{Log: l, Index: -1})
	}

	return NewMultiReaderFrom(ch, logs...)
}

// NewMultiReaderFrom creates composite reader of specified readers.
// Order of readers matters in a way how Events are deduplicated.
// Initial state:
// - InMemory [A, B] - as im.A, im.B
// R1: [C,D,E] - as r1.C... - pointer to D
// R2: [A,D,F] - pointer to A
// R3: [B, G] - pointer to B
// result [im.A, im.B, r1.E, r2.D, r2.F, r3.G]
func NewMultiReaderFrom(ch storage.Checker, l ...PointedLog) Reader {
	return &multiReader{
		logs:    l,
		checker: ch,
	}
}

func (mr *multiReader) Read() []Event {
	events := make([]Event, 0)

	for _, r := range mr.logs {
		ee := r.Log.Read()
		for _, e := range ee {
			if exists(e, events, mr.checker) {
				continue
			}
			events = append(events, e)
		}
	}

	return events
}

func (mr *multiReader) ReadFrom(index int64) []Event {
	events := make([]Event, 0)

	for _, r := range mr.logs {
		var ee []Event

		if r.Index == -1 {
			ee = r.Log.Read()
		} else {
			ee = r.Log.ReadFrom(r.Index)
		}

		for _, e := range ee {
			if exists(e, events, mr.checker) {
				continue
			}
			events = append(events, e)
		}
	}

	return events
}

func exists(event Event, log []Event, checker storage.Checker) bool {
	for _, e := range log {
		if e.Module == event.Module && e.Version == event.Version {
			return true
		}
	}

	return checker.Exists(event.Module, event.Version)
}
