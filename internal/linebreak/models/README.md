# BudouX models

`ja.json` and `zh-hans.json` are BudouX's Japanese and Simplified Chinese models, copied unchanged from [google/budoux](https://github.com/google/budoux) at v0.10.4 (`budoux/models/`). They are under the Apache License 2.0, in `LICENSE` here. `../budoux.go` reads them as BudouX's `parser.py` does.

To update them, copy the two files from a newer release and run `go test ./internal/linebreak`: `TestPhrasesAsBudouX` holds BudouX's own examples.
