// Package money holds the arithmetic the ledger depends on: splitting a cost
// across participants, and valuing shuttles drawn from stock.
//
// Nothing here touches the database, and nothing here should. Constitution
// Principle VIII asks that `go test ./...` stay green on a clean checkout, and
// these are the rules most worth testing exhaustively — every awkward
// participant count, every total that refuses to divide evenly. Keeping them
// free of infrastructure means those tests run everywhere, every time, in
// milliseconds.
//
// Two rules from the constitution shape everything in this package:
//
//   - Principle V: money is int64 cents. There is no float64 in this package
//     and there must never be one. A cost that does not divide evenly is
//     resolved by distributing the remainder, not by rounding each share.
//
//   - A $50 tube of twelve shuttles is 416.66... cents each. That number is
//     never stored. Stock carries a value and a unit count, and the cost of
//     what is consumed is derived from the pair at the moment of consumption.
package money
