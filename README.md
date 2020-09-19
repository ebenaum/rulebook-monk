## Rulebook lexer and parser

`make rulebook SRC=<path of rulebook source>`

as lib:
```go
  type BuilderConfig struct {
	  TableOfContents bool
  }

  func Build(input io.Reader, w io.Writer, config BuilderConfig) error
```
