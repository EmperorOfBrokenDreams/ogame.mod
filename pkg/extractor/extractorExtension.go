package extractor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/alaingilbert/ogame/pkg/ogame"
)

type ExtractorMessagesBytes interface {
	ExtractMessages(pageHTML []byte) ([]ogame.Message, int64, error)
}

type ExtractorMessagesDoc interface {
	ExtractMessagesFromDoc(doc *goquery.Document) ([]ogame.Message, int64, error)
}

type ExtractMessagesBytesDoc interface {
	ExtractorMessagesBytes
	ExtractorMessagesDoc
}
