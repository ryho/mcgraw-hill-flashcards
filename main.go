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
	DesiredLanguage = "Spanish"
	DesiredBookName = "Complete Spanish Step-by-Step"

	DownloadAllBooks = false
)

const (
	// These are for the program and should not be changed.
	GetMenuUrl          = "https://mhe-language-lab.azurewebsites.net/api/GetSubMenus?parentID=%d"
	GetFlashCardsUrl    = "https://mhe-language-lab.azurewebsites.net/api/GetFlashCards?menuID=%d"
	FlashCards          = "Flashcards"
	ProgressChecks      = "Progress Checks"
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
	languageMenu, err := GetMenuOptions(0)
	if err != nil {
		return err
	}
	for _, languageEntry := range languageMenu {
		if DownloadAllBooks || languageEntry.MenuTitle == DesiredLanguage {
			fmt.Printf("Downloading flashcards for language %s\n", languageEntry.MenuTitle)
			err = DownloadFlashCardsForLanguage(languageEntry)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func DownloadFlashCardsForLanguage(language *MenuEntry) error {
	// Get the list of books for the selected language and select the desired book.
	bookMenu, err := GetMenuOptions(language.MenuID)
	if err != nil {
		return err
	}
	for _, bookEntry := range bookMenu {
		if DownloadAllBooks || bookEntry.MenuTitle == DesiredBookName {
			fmt.Printf("Downloading flashcards for book %s\n", bookEntry.MenuTitle)
			err = DownloadFlashCardsForBook(language, bookEntry)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DownloadFlashCardsForBook(language, book *MenuEntry) error {
	// Get the chapters of the book and download the flashcards.
	// For each book, there are multiple options. Ones called "Flashcards" and "Progress Checks" are compatible with flashcards.
	// There are some books that don't have either of those. In that case, the book is not compatible with this program.
	flashCardOption, err := GetMenuOptionWithNames(book.MenuID, FlashCards, ProgressChecks)
	if err != nil {
		return err
	}
	if flashCardOption == nil {
		fmt.Printf("Book %s does not have flashcards or progress checks\n", book.MenuTitle)
		return nil
	}

	// Flash cards will return a list of chapters. Which might have subchapters or may lead directly to flashcards.
	chapters, err := GetMenuOptions(flashCardOption.MenuID)
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
		// Download the flashcards for the whole chapter, including subchapters.
		chapterCards, err := GetChapterCards(chapter)
		if err != nil {
			return err
		}
		// Make 1-digit numbers start with zero, so that alphabetical sorting works correctly.
		if chapter.MenuTitle[1] == '.' {
			chapter.MenuTitle = "0" + chapter.MenuTitle
		}
		// Strip html tags from the chapter title
		chapter.MenuTitle = strings.ReplaceAll(chapter.MenuTitle, "<i>", "")
		chapter.MenuTitle = strings.ReplaceAll(chapter.MenuTitle, "</i>", "")

		fmt.Printf("Chapter: %v\n", chapter.MenuTitle)

		// Write the flashcards to the file, one for Spanish to English and one for English to Spanish.
		for _, card := range chapterCards {
			title := chapter.MenuTitle
			if chapter.MenuTitle[2:4] == ". " {
				title = chapter.MenuTitle[0:4] + "(" + language.MenuTitle[0:1] + "2E) " + chapter.MenuTitle[4:]
			}
			fileOutput.WriteString(fmt.Sprintf("%s\t%s\t%s %s\n", card.SideA, card.SideB, book.MenuTitle, title))
		}
		for _, card := range chapterCards {
			title := chapter.MenuTitle
			if chapter.MenuTitle[2:4] == ". " {
				title = chapter.MenuTitle[0:4] + "(E2" + language.MenuTitle[0:1] + ") " + chapter.MenuTitle[4:]
			}
			fileOutput.WriteString(fmt.Sprintf("%s\t%s\t%s %s\n", card.SideB, card.SideA, book.MenuTitle, title))
		}
	}
	if fileOutput.Len() == len(FileStart) {
		fmt.Printf("No flashcards found for book %s\n", book.MenuTitle)
		return nil
	} else {
		fileName := fmt.Sprintf("output/%s.txt", book.MenuTitle)
		err = os.WriteFile(fileName, []byte(fileOutput.String()), 0644)
		if err != nil {
			return err
		}

		fmt.Printf("File written to %s\n", fileName)
	}

	return nil
}

func GetChapterCards(chapter *MenuEntry) ([]*Card, error) {
	fmt.Printf("  %+v\n", chapter.MenuTitle)
	var chapterCards []*Card
	if chapter.FlashCardsAndQuiz {
		// If this chapter is the bottom of the graph, get the flashcards.
		flashCardMode, err := GetMenuOptionWithNames(chapter.MenuID, FlashCardsStudyMode)
		if err != nil {
			return nil, err
		}
		if flashCardMode == nil {
			fmt.Printf("Chapter %s does not have flashcard mode", chapter.MenuTitle)
			return nil, nil
		}
		cards, err := GetFlashCards(flashCardMode.MenuID)
		if err != nil {
			return nil, err
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
			} else {
				card.SideA = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(card.SideA, "\t", ""), "\n", ""), "\r", ""), "\n", "")
				card.SideB = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(card.SideB, "\t", ""), "\n", ""), "\r", ""), "\n", "")
			}

			chapterCards = append(chapterCards, card)
		}
	} else {
		// If this chapter has subchapters, get the flashcards for each subchapter.
		subChapters, err := GetMenuOptions(chapter.MenuID)
		if err != nil {
			return nil, err
		}
		for _, subChapter := range subChapters {
			subChapterCards, err := GetChapterCards(subChapter)
			if err != nil {
				return nil, err
			}
			chapterCards = append(chapterCards, subChapterCards...)
		}
	}
	return chapterCards, nil
}

func GetMenuOptions(parentID int) ([]*MenuEntry, error) {
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

func GetMenuOptionWithNames(parentID int, names ...string) (*MenuEntry, error) {
	options, err := GetMenuOptions(parentID)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		for _, name := range names {
			if option.MenuTitle == name {
				return option, nil
			}
		}
	}
	return nil, nil
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
