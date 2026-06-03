// Package prompts holds pindex's LLM prompts — ported verbatim from PageIndex's
// inline strings — plus the typed output schemas each one expects. Keeping the
// "thinking" chain-of-thought fields is intentional: they carry accuracy, so we
// validate-then-retry rather than constrain decoding (see internal/llm).
package prompts

import "fmt"

// TOCItem is one node emitted by the structure-generation prompts. PhysicalIndex
// arrives as "<physical_index_N>" (a string) and is converted to an int later.
type TOCItem struct {
	Structure     string `json:"structure"`
	Title         string `json:"title"`
	PhysicalIndex string `json:"physical_index"`
}

// StartBegin is the reply schema for CheckTitleAppearanceInStart.
type StartBegin struct {
	Thinking   string `json:"thinking"`
	StartBegin string `json:"start_begin"` // "yes" | "no"
}

// Appearance is the reply schema for CheckTitleAppearance (verification).
type Appearance struct {
	Thinking string `json:"thinking"`
	Answer   string `json:"answer"` // "yes" | "no"
}

// TOCDetected is the reply schema for TOCDetector.
type TOCDetected struct {
	Thinking    string `json:"thinking"`
	TOCDetected string `json:"toc_detected"` // "yes" | "no"
}

// GenerateTOCInit asks the model to emit the initial hierarchical structure for
// the first page-group of a document with no usable table of contents.
func GenerateTOCInit(part string) string {
	return `
    You are an expert in extracting hierarchical tree structure, your task is to generate the tree structure of the document.

    The structure variable is the numeric system which represents the index of the hierarchy section in the table of contents. For example, the first section has structure index 1, the first subsection has structure index 1.1, the second subsection has structure index 1.2, etc.

    For the title, you need to extract the original title from the text, only fix the space inconsistency.

    The provided text contains tags like <physical_index_X> and <physical_index_X> to indicate the start and end of page X.

    For the physical_index, you need to extract the physical index of the start of the section from the text. Keep the <physical_index_X> format.

    The response should be in the following format.
        [
            {
                "structure": <structure index, "x.x.x"> (string),
                "title": <title of the section, keep the original title>,
                "physical_index": "<physical_index_X> (keep the format)"
            },

        ],


    Directly return the final JSON structure. Do not output anything else.` +
		"\nGiven text\n:" + part
}

// GenerateTOCContinue extends an existing structure with the next page-group.
func GenerateTOCContinue(prevTOCJSON, part string) string {
	return `
    You are an expert in extracting hierarchical tree structure.
    You are given a tree structure of the previous part and the text of the current part.
    Your task is to continue the tree structure from the previous part to include the current part.

    The structure variable is the numeric system which represents the index of the hierarchy section in the table of contents. For example, the first section has structure index 1, the first subsection has structure index 1.1, the second subsection has structure index 1.2, etc.

    For the title, you need to extract the original title from the text, only fix the space inconsistency.

    The provided text contains tags like <physical_index_X> and <physical_index_X> to indicate the start and end of page X.

    For the physical_index, you need to extract the physical index of the start of the section from the text. Keep the <physical_index_X> format.

    The response should be in the following format.
        [
            {
                "structure": <structure index, "x.x.x"> (string),
                "title": <title of the section, keep the original title>,
                "physical_index": "<physical_index_X> (keep the format)"
            },
            ...
        ]

    Directly return the additional part of the final JSON structure. Do not output anything else.` +
		"\nGiven text\n:" + part + "\nPrevious tree structure\n:" + prevTOCJSON
}

// CheckTitleAppearanceInStart asks whether a section begins at the very top of a
// page (which shifts the previous section's end index by one).
func CheckTitleAppearanceInStart(title, pageText string) string {
	return fmt.Sprintf(`
    You will be given the current section title and the current page_text.
    Your job is to check if the current section starts in the beginning of the given page_text.
    If there are other contents before the current section title, then the current section does not start in the beginning of the given page_text.
    If the current section title is the first content in the given page_text, then the current section starts in the beginning of the given page_text.

    Note: do fuzzy matching, ignore any space inconsistency in the page_text.

    The given section title is %s.
    The given page_text is %s.

    reply format:
    {
        "thinking": <why do you think the section appears or starts in the page_text>
        "start_begin": "yes or no" (yes if the section starts in the beginning of the page_text, no otherwise)
    }
    Directly return the final JSON structure. Do not output anything else.`, title, pageText)
}

// CheckTitleAppearance asks whether a section title appears on a given page —
// used to verify generated physical indices.
func CheckTitleAppearance(title, pageText string) string {
	return fmt.Sprintf(`
    Your job is to check if the given section appears or starts in the given page_text.

    Note: do fuzzy matching, ignore any space inconsistency in the page_text.

    The given section title is %s.
    The given page_text is %s.

    Reply format:
    {

        "thinking": <why do you think the section appears or starts in the page_text>
        "answer": "yes or no" (yes if the section appears or starts in the page_text, no otherwise)
    }
    Directly return the final JSON structure. Do not output anything else.`, title, pageText)
}

// TOCDetector asks whether a page contains a table of contents.
func TOCDetector(content string) string {
	return fmt.Sprintf(`
    Your job is to detect if there is a table of content provided in the given text.

    Given text: %s

    return the following JSON format:
    {
        "thinking": <why do you think there is a table of content in the given text>
        "toc_detected": "<yes or no>",
    }

    Directly return the final JSON structure. Do not output anything else.
    Please note: abstract,summary, notation list, figure list, table list, etc. are not table of contents.`, content)
}

// NodeSummary asks for a short description of a section's text. Returns plain text.
func NodeSummary(text string) string {
	return fmt.Sprintf(`You are given a part of a document, your task is to generate a description of the partial document about what are main points covered in the partial document.

Partial Document Text: %s

Directly return the description, do not include any other text.`, text)
}

// DocDescription asks for a one-sentence document description from its structure.
// Returns plain text.
func DocDescription(structureJSON string) string {
	return fmt.Sprintf(`Your are an expert in generating descriptions for a document.
You are given a structure of a document. Your task is to generate a one-sentence description for the document, which makes it easy to distinguish the document from other documents.

Document Structure: %s

Directly return the description, do not include any other text.`, structureJSON)
}

// PageSelection is the reply schema for AskSelectPages.
type PageSelection struct {
	Thinking string `json:"thinking"`
	Pages    string `json:"pages"` // a page selector like "5-7,12"
}

// AnswerOut is the reply schema for AskAnswer.
type AnswerOut struct {
	Thinking  string `json:"thinking"`
	Answer    string `json:"answer"`
	PagesUsed string `json:"pages_used"`
}

// AskSelectPages asks the model to pick the tightest page ranges likely to hold
// the answer, given the text-stripped structure (the tree-search step).
func AskSelectPages(structure, question string) string {
	return fmt.Sprintf(`You are navigating a document to answer a question. You are given the document's
hierarchical structure (titles, summaries, and page ranges — no full text) and a question.
Choose the TIGHTEST set of page ranges most likely to contain the answer. Prefer a few pages over many.

Document structure:
%s

Question: %s

Reply JSON:
{ "thinking": <which sections are relevant and why>, "pages": "<page ranges like 5-7,12>" }
Directly return the JSON. Do not output anything else.`, structure, question)
}

// AskAnswer asks the model to answer strictly from the fetched page content and
// cite the pages it used.
func AskAnswer(question, pagesJSON string) string {
	return fmt.Sprintf(`Answer the question using ONLY the provided page content. Cite the page numbers you used.
If the answer is not present in the pages, say you cannot find it.

Question: %s

Pages (JSON list of {page, content}):
%s

Reply JSON:
{ "thinking": <reasoning>, "answer": <concise answer>, "pages_used": "<page numbers like 5,7>" }
Directly return the JSON. Do not output anything else.`, question, pagesJSON)
}
