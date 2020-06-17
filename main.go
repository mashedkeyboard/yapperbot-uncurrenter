package main

//
// Uncurrenter, the {{current}} tag removal bot for Wikipedia
// Copyright (C) 2020 Naypta

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

import (
	"log"
	"regexp"
	"time"

	"cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/mashedkeyboard/ybtools"
)

var currentTemplateRegex *regexp.Regexp

func main() {
	ybtools.SetupBot("Uncurrenter", "Yapperbot")
	defer ybtools.SaveEditLimit()

	currentTemplateRegex = regexp.MustCompile(`(?i){{current *(?:\|(?:{{[^}{]*}}|[^}{]*)*|)}}\n?`)

	w := ybtools.CreateAndAuthenticateClient()

	parameters := params.Values{
		"action":         "query",
		"prop":           "revisions",
		"generator":      "embeddedin",
		"geititle":       "Template:Current",
		"geinamespace":   "0",
		"geifilterredir": "nonredirects",
		"rvprop":         "timestamp|content",
		"rvslots":        "main",
	}

	query := w.NewQuery(parameters)
	for query.Next() {
		pages := ybtools.GetPagesFromQuery(query.Resp())
		if len(pages) > 0 {
			for _, page := range pages {
				pageTitle, err := page.GetString("title")
				if err != nil {
					log.Println("Failed to get title from page, so skipping it. Error was", err)
					continue
				}

				pageContent, err := ybtools.GetContentFromPage(page)
				if err != nil {
					log.Println("Failed to get content from page", pageTitle, ", so skipping it. Error was", err)
					continue
				}

				pageRevisions, err := page.GetObjectArray("revisions")
				if err != nil {
					log.Println("Failed to get revisions array from page, so skipping it. Error was", err)
					continue
				}
				lastTimestamp, err := pageRevisions[0].GetString("timestamp")
				if err != nil {
					log.Println("Failed to get timestamp from revision, so skipping the page. Error was", err)
					continue
				}
				lastTimestampProcessed, err := time.Parse(time.RFC3339, lastTimestamp)
				if err != nil {
					log.Println("Failed to parse last revision timestamp, so skipping the page. Error was", err)
					continue
				}

				// if it's been more than five hours since the last edit, and we can edit it
				if time.Now().Sub(lastTimestampProcessed).Hours() > 5 && ybtools.BotAllowed(pageContent) && ybtools.CanEdit() {
					newPageContent := currentTemplateRegex.ReplaceAllString(pageContent, "")
					err = w.Edit(params.Values{
						"title":    pageTitle,
						"text":     newPageContent,
						"summary":  "Removing the {{current}} template as the article hasn't been edited in over five hours. Did I get this wrong? Please revert me and check my userpage!",
						"notminor": "true",
						"bot":      "true",
					})
					if err == nil {
						log.Println("Successfully removed current template from", pageTitle)
					} else {
						switch err.(type) {
						case mwclient.APIError:
							if err.(mwclient.APIError).Code == "editconflict" {
								log.Println("Edit conflicted on page", pageTitle, "assuming it's still active and skipping")
								continue
							} else {
								ybtools.PanicErr("API error raised, can't handle, so failing. Error was ", err)
							}
						default:
							ybtools.PanicErr("Non-API error raised, can't handle, so failing. Error was ", err)
						}
					}
				}
			}
		}
	}
}
