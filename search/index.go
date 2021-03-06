package search

import (
	"database/sql"
	"log"
	"strconv"
	"time"

	"github.com/blevesearch/bleve"
	_ "github.com/blevesearch/bleve/analysis/analyzer/keyword" // required for keyword analysis
	"github.com/kaaryasthan/kaaryasthan/config"
	"github.com/lib/pq"
)

const keyword = "keyword"

// IndexRepository to manage search
type IndexRepository interface {
	GenerateFromDatabase() error
	SubscribeAndCreateIndex()
}

// BleveIndex implements the repository interface
type BleveIndex struct {
	db   *sql.DB
	conf config.Configuration
	Idx  bleve.Index
}

// Item to index
type Item struct {
	Num            int        `json:"num"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Comments       []string   `json:"comment"`
	Project        string     `json:"project"`
	Labels         []string   `json:"label"`
	Milestones     []string   `json:"milestone"`
	Author         string     `json:"author"`
	Editor         string     `json:"editor"`
	Created        time.Time  `json:"created"`
	Updated        *time.Time `json:"updated"`
	Assignees      []string   `json:"assignee"`
	Subscribers    []string   `json:"subscriber"`
	State          string     `json:"state"`
	Mentions       []string   `json:"mention"`
	CommentAuthors []string   `json:"comment_author"`
	CommentEditors string     `json:"comment_editor"`
	CommentCreated time.Time  `json:"comment_created"`
	CommentUpdated *time.Time `json:"comment_updated"`
}

// Type of the document for custom mapping
func (i *Item) Type() string {
	return "item"
}

// GenerateFromDatabase creates full-text search index
func (bi *BleveIndex) GenerateFromDatabase() error {
	rows, err := bi.db.Query("SELECT num, title, description, labels, open_state FROM items")
	for rows.Next() {
		var l pq.StringArray
		var state bool
		d := Item{}
		err = rows.Scan(&d.Num, &d.Title, &d.Description, &l, &state)
		if err != nil {
			return err
		}
		d.Labels = []string(l)
		d.State = "closed"
		if state {
			d.State = "open"
		}
		if err = bi.Idx.Index(strconv.Itoa(d.Num), d); err != nil {
			log.Println("Error indexing:", err)
		}
	}
	return err
}

func (bi *BleveIndex) waitForNotification(l *pq.Listener) {
	// nolint: megacheck
	for {
		select {
		case n := <-l.Notify:
			if n == nil {
				continue
			}
			d := Item{}
			id, err := strconv.Atoi(n.Extra)
			if err != nil {
				continue
			}
			var l pq.StringArray
			var state bool
			if err := bi.db.QueryRow("SELECT num, title, description, labels, open_state FROM items WHERE id = $1",
				id).Scan(&d.Num, &d.Title, &d.Description, &l, &state); err != nil {
				log.Println("Error running query:", err)
				continue
			}
			d.Labels = []string(l)
			d.State = "closed"
			if state {
				d.State = "open"
			}
			if err := bi.Idx.Index(strconv.Itoa(d.Num), &d); err != nil {
				log.Println("Error indexing:", err)
			}
		}
	}

}

// SubscribeAndCreateIndex creates full-text search index
func (bi *BleveIndex) SubscribeAndCreateIndex() *pq.Listener {
	reportListenProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Println(err.Error())
		}
	}
	listener := pq.NewListener(bi.conf.PostgresConfig(), 1*time.Second, 5*time.Minute, reportListenProblem)

	if err := listener.Listen("item_change"); err != nil {
		if err := listener.Close(); err != nil {
			log.Println("Error closing the database listener:", err)
		}
	}
	go bi.waitForNotification(listener)
	return listener
}

// NewBleveIndex constructs a new repository
func NewBleveIndex(db *sql.DB, conf config.Configuration) *BleveIndex {
	idx, err := bleve.Open(conf.BleveIndexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		idxMapping := bleve.NewIndexMapping()
		docMapping := bleve.NewDocumentMapping()

		projectFieldMapping := bleve.NewTextFieldMapping()
		projectFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("project", projectFieldMapping)

		labelFieldMapping := bleve.NewTextFieldMapping()
		labelFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("label", labelFieldMapping)

		milestoneFieldMapping := bleve.NewTextFieldMapping()
		milestoneFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("milestone", milestoneFieldMapping)

		authorFieldMapping := bleve.NewTextFieldMapping()
		authorFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("author", authorFieldMapping)

		stateFieldMapping := bleve.NewTextFieldMapping()
		stateFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("state", stateFieldMapping)

		editorFieldMapping := bleve.NewTextFieldMapping()
		editorFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("editor", editorFieldMapping)

		assigneeFieldMapping := bleve.NewTextFieldMapping()
		assigneeFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("assignee", assigneeFieldMapping)

		subscribeFieldMapping := bleve.NewTextFieldMapping()
		subscribeFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("subscribe", subscribeFieldMapping)

		mentionFieldMapping := bleve.NewTextFieldMapping()
		mentionFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("mention", mentionFieldMapping)

		comAuthorFieldMapping := bleve.NewTextFieldMapping()
		comAuthorFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("comAuthor", comAuthorFieldMapping)

		comEditorFieldMapping := bleve.NewTextFieldMapping()
		comEditorFieldMapping.Analyzer = keyword
		docMapping.AddFieldMappingsAt("comEditor", comEditorFieldMapping)

		idxMapping.AddDocumentMapping("item", docMapping)
		idx, err = bleve.New(conf.BleveIndexPath, idxMapping)
		if err != nil {
			log.Println("Error creating index", err)
		}
	}
	return &BleveIndex{db, conf, idx}
}
