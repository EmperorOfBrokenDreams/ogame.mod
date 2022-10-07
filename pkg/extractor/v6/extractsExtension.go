package v6

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/alaingilbert/ogame/pkg/ogame"
)

// ExtractMessages ...
func (e *Extractor) ExtractMessages(pageHTML []byte) ([]ogame.Message, int64, error) {
	panic("implement me")
}

// ExtractMessagesFromDoc ...
func (e *Extractor) ExtractMessagesFromDoc(doc *goquery.Document) ([]ogame.Message, int64, error) {
	panic("implement me")
}
