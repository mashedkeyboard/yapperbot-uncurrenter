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
	"crypto/md5"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/mashedkeyboard/ybtools/v2"
)

var currentTemplateRegex *regexp.Regexp

func main() {
	ybtools.SetupBot(ybtools.BotSettings{TaskName: "Uncurrenter", BotUser: "Yapperbot"})
	defer ybtools.SaveEditLimit()

	w := ybtools.CreateAndAuthenticateClient(ybtools.DefaultMaxlag)

	// Check for every redirect to the {{current}} template, and include all of those - these will show
	// as transclusions of the template, and are covered under the BRFA as they are the same template
	queryRedirects := w.NewQuery(params.Values{
		"action":       "query",
		"generator":    "linkshere",
		"titles":       "Template:Current",
		"glhprop":      "title",
		"glhnamespace": "10",
		"glhshow":      "redirect",
	})

	var regexBuilder strings.Builder
	regexBuilder.WriteString(`(?i){{(?:current`)

	for queryRedirects.Next() {
		pages := ybtools.GetPagesFromQuery(queryRedirects.Resp())
		if len(pages) > 0 {
			for _, page := range pages {
				pageTitle, err := page.GetString("title")
				if err != nil {
					log.Println("Failed to get title from redirect page for template, so skipping it. Error was", err)
					continue
				}
				regexBuilder.WriteString("|")
				regexBuilder.WriteString(regexp.QuoteMeta(strings.TrimPrefix(pageTitle, "Template:")))
			}
		}
	}
	regexBuilder.WriteString(`) *(?:\|(?:{{[^}{]*}}|[^}{]*)*|)}}\n?`)

	currentTemplateRegex = regexp.MustCompile(regexBuilder.String())

	ybtools.ForPageInQuery(params.Values{
		"action":         "query",
		"prop":           "revisions",
		"generator":      "embeddedin",
		"geititle":       "Template:Current",
		"geinamespace":   "0",
		"geifilterredir": "nonredirects",
		"rvprop":         "timestamp|content",
		"rvslots":        "main",
		"curtimestamp":   "1",
	}, func(pageTitle, pageContent, revTS, curTS string) {
		revTSProcessed, err := time.Parse(time.RFC3339, revTS)
		if err != nil {
			log.Println("Failed to parse last revision timestamp, so skipping the page. Error was", err)
			return
		}

		// if it's been more than five hours since the last edit, and we can edit it
		if time.Now().Sub(revTSProcessed).Hours() > 5 && ybtools.BotAllowed(pageContent) && ybtools.CanEdit() {
			newPageContent := currentTemplateRegex.ReplaceAllString(pageContent, "")
			if newPageContent == pageContent {
				log.Println("newPageContent was the same as pageContent on page", pageTitle, "so ignoring")
				return
			}

			err = w.Edit(params.Values{
				"title":          pageTitle,
				"text":           newPageContent,
				"md5":            fmt.Sprintf("%x", md5.Sum([]byte(newPageContent))),
				"summary":        "Auto-removing {{current}} - no edits in 5hrs+. The event may still be current, but [[Template:Current|the {{current}} template is designed only for articles which many editors are editing, and is usually up for less than a day]].",
				"notminor":       "true",
				"bot":            "true",
				"basetimestamp":  revTS,
				"starttimestamp": curTS,
			})
			if err == nil {
				log.Println("Successfully removed current template from", pageTitle)
			} else {
				switch err.(type) {
				case mwclient.APIError:
					if err.(mwclient.APIError).Code == "editconflict" {
						log.Println("Edit conflicted on page", pageTitle, "assuming it's still active and skipping")
						return
					}

					ybtools.PanicErr("API error raised, can't handle, so failing. Error was ", err)
				default:
					ybtools.PanicErr("Non-API error raised, can't handle, so failing. Error was ", err)
				}
			}
		}
	})
}
