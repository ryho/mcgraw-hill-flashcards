package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	// These should be set to control the book that is downloaded.
	Language = "Spanish"
	BookName = "Complete Spanish Step-by-Step"
)

const (
	// These are for the program and should not be changed.
	GetMenuUrl          = "https://mhe-language-lab.azurewebsites.net/api/GetSubMenus?parentID=%d"
	GetFlashCardsUrl    = "https://mhe-language-lab.azurewebsites.net/api/GetFlashCards?menuID=%d"
	FlashCards          = "Flashcards"
	FlashCardsStudyMode = "Flashcards: Study Mode"
	FileStart           = `#separator:tab
#html:true
#deck column:3
`
)

func main() {
	err := DoEverything()
	if err != nil {
		fmt.Println(err)
	}
}
func DoEverything() error {
	// Get the list of languages and select the desired language.
	languageMenu, err := GetMenu(0)
	if err != nil {
		return err
	}
	var languageId int
	for _, languageEntry := range languageMenu {
		if languageEntry.MenuTitle == Language {
			languageId = languageEntry.MenuID
			break
		}
	}

	// Get the list of books for the selected language and select the desired book.
	bookMenu, err := GetMenu(languageId)
	if err != nil {
		return err
	}
	var bookId int
	for _, bookEntry := range bookMenu {
		if bookEntry.MenuTitle == BookName {
			bookId = bookEntry.MenuID
			break
		}
	}

	// Get the chapters of the book and download the flashcards.
	bookOptions, err := GetMenu(bookId)
	if err != nil {
		return err
	}
	var flashCardId int
	for _, options := range bookOptions {
		if options.MenuTitle == FlashCards {
			flashCardId = options.MenuID
			break
		}
	}
	chapters, err := GetMenu(flashCardId)
	if err != nil {
		return err
	}
	err = os.MkdirAll("output", 0755)
	if err != nil {
		return err
	}

	fileOutput := strings.Builder{}
	fileOutput.WriteString(FileStart)
	for _, chapter := range chapters {
		// Make 1-digit numbers start with zero, so that alphabetical sorting works correctly.
		if chapter.MenuTitle[1] == '.' {
			chapter.MenuTitle = "0" + chapter.MenuTitle
		}
		// Strip html tags from the chapter title
		chapter.MenuTitle = strings.ReplaceAll(chapter.MenuTitle, "<i>", "")
		chapter.MenuTitle = strings.ReplaceAll(chapter.MenuTitle, "</i>", "")

		fmt.Printf("%+v\n", chapter.MenuTitle)
		// Get all sections in the chapter and get the study mode flashcards in each section.
		sections, err := GetMenu(chapter.MenuID)
		if err != nil {
			return err
		}
		var chapterCards []*Card
		for _, section := range sections {
			fmt.Printf("  %+v\n", section.MenuTitle)
			modes, err := GetMenu(section.MenuID)
			if err != nil {
				return err
			}
			for _, mode := range modes {
				if mode.MenuTitle == FlashCardsStudyMode {
					cards, err := GetFlashCards(mode.MenuID)
					if err != nil {
						return err
					}
					for _, card := range cards {
						if strings.Contains(card.SideA, "\r\n") {
							// There is a broken card in the Spanish course that totally mangles the output.
							// Handle that card correctly. There are several cards combined into one message, this will
							// actually put them in the output correctly.
							parts := strings.Split(card.SideA, "\r\n")
							for _, part := range parts[1:] {
								aAndB := strings.Split(part, "\t")
								if len(aAndB) == 2 {
									chapterCards = append(chapterCards, &Card{
										SideA: aAndB[0],
										SideB: aAndB[1],
									})
								}
							}
							continue
						}
						chapterCards = append(chapterCards, card)
					}
				}
			}
		}
		if len(chapterCards) == 0 {
			continue
		}
		for _, card := range chapterCards {
			title := chapter.MenuTitle
			if chapter.MenuTitle[2:4] == ". " {
				title = chapter.MenuTitle[0:4] + "(" + Language[0:1] + "2E) " + chapter.MenuTitle[4:]
			}
			fileOutput.WriteString(fmt.Sprintf("%s\t%s\t%s %s\n", card.SideA, card.SideB, BookName, title))
		}
		for _, card := range chapterCards {
			title := chapter.MenuTitle
			if chapter.MenuTitle[2:4] == ". " {
				title = chapter.MenuTitle[0:4] + "(E2" + Language[0:1] + ") " + chapter.MenuTitle[4:]
			}
			fileOutput.WriteString(fmt.Sprintf("%s\t%s\t%s %s\n", card.SideB, card.SideA, BookName, title))
		}
	}
	fileName := fmt.Sprintf("output/%s.tsv", BookName)
	err = os.WriteFile(fileName, []byte(fileOutput.String()), 0644)
	if err != nil {
		return err
	}

	fmt.Printf("File written to %s\n", fileName)

	return nil
}
func GetMenu(parentID int) ([]*MenuEntry, error) {
	resp, err := http.Get(fmt.Sprintf(GetMenuUrl, parentID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var getMenuResponse []*MenuEntry
	if err := json.Unmarshal(body, &getMenuResponse); err != nil {
		return nil, err
	}
	return getMenuResponse, nil
}

func GetFlashCards(menuID int) ([]*Card, error) {
	resp, err := http.Get(fmt.Sprintf(GetFlashCardsUrl, menuID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var getFlashCardsResponse []*Card
	if err := json.Unmarshal(body, &getFlashCardsResponse); err != nil {
		return nil, err
	}
	return getFlashCardsResponse, nil
}

type MenuEntry struct {
	MenuID            int    `json:"Menu_ID"`
	MenuTitle         string `json:"MenuTitle"`
	TitleInformation  string `json:"TitleInformation"`
	Base64Image       string `json:"Base64Image"`
	MenuFormat        string `json:"MenuFormat"`
	DeckType          string `json:"DeckType"`
	SelfScoring       bool   `json:"SelfScoring"`
	DeckTitle         string `json:"DeckTitle"`
	FlashCardsAndQuiz bool   `json:"FlashCardsAndQuiz"`
	SideALabel        string `json:"SideALabel"`
	SideBLabel        string `json:"SideBLabel"`
	DataDeckID        int    `json:"DataDeck_ID"`
	ForceSideA        bool   `json:"ForceSideA"`
	Unpublished       bool   `json:"Unpublished"`
}

type Card struct {
	CardID     int    `json:"Card_ID"`
	SideA      string `json:"SideA"`
	SideB      string `json:"SideB"`
	StyleA     string `json:"StyleA"`
	StyleB     string `json:"StyleB"`
	SideAAudio string `json:"SideAAudio"`
	SideBAudio string `json:"SideBAudio"`
	SideAImage string `json:"SideAImage"`
	SideBImage string `json:"SideBImage"`
	SideAVideo string `json:"SideAVideo"`
	SideBVideo string `json:"SideBVideo"`
	SideALabel string `json:"SideALabel"`
	SideBLabel string `json:"SideBLabel"`
	TTSAudio   bool   `json:"TTSAudio"`
	TTSSideA   string `json:"TTSSideA"`
	TTSSideB   string `json:"TTSSideB"`
}
