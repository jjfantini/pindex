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

// PageIndexGiven is the reply schema for DetectPageIndex.
type PageIndexGiven struct {
	Thinking            string `json:"thinking"`
	PageIndexGivenInTOC string `json:"page_index_given_in_toc"` // "yes" | "no"
}

// TOCPageEntry is one transformed TOC line: a dotted structure code, a title, and
// the PRINTED page label (nil when the line has no page number).
type TOCPageEntry struct {
	Structure string `json:"structure"`
	Title     string `json:"title"`
	Page      *int   `json:"page"`
}

// TOCTransformOut wraps the transformer's reply.
type TOCTransformOut struct {
	TableOfContents []TOCPageEntry `json:"table_of_contents"`
}

// DetectPageIndex asks whether a table of contents lists page numbers.
func DetectPageIndex(tocContent string) string {
	return fmt.Sprintf(`You will be given a table of contents.

Your job is to detect if there are page numbers/indices given within the table of contents.

Given text: %s

Reply format:
{
    "thinking": <why do you think there are page numbers/indices given within the table of contents>
    "page_index_given_in_toc": "<yes or no>"
}
Directly return the final JSON structure. Do not output anything else.`, tocContent)
}

// TOCTransform converts a raw table of contents into structured JSON entries with
// their printed page numbers.
func TOCTransform(tocContent string) string {
	return fmt.Sprintf(`You are given a table of contents. Your job is to transform the whole table of contents
into a JSON format included in table_of_contents.

structure is the numeric system which represents the index of the hierarchy section in the table of
contents. For example, the first section has structure index 1, the first subsection has structure
index 1.1, the second subsection has structure index 1.2, etc.

The response should be in the following JSON format:
{
    "table_of_contents": [
        {
            "structure": <structure index, "x.x.x" or null> (string),
            "title": <title of the section>,
            "page": <page number or null>
        },
        ...
    ]
}
Transform the full table of contents in one go. Directly return the final JSON structure, do not
output anything else.

Table of contents:
%s`, tocContent)
}

// TOCIndexExtract maps TOC titles to physical page indices using tagged pages.
func TOCIndexExtract(tocJSON, taggedPages string) string {
	return fmt.Sprintf(`You are given a table of contents in JSON format and several pages of a document. Your
job is to add the physical_index to the table of contents.

The provided pages contain tags like <physical_index_X> and <physical_index_X> to indicate the
physical location of page X.

The structure variable is the numeric hierarchy index (e.g. 1, 1.1, 1.2).

The response should be in the following JSON format:
[
    {
        "structure": <structure index, "x.x.x" or null> (string),
        "title": <title of the section>,
        "physical_index": "<physical_index_X>" (keep the format)
    },
    ...
]

Only add the physical_index to sections that are in the provided pages. If a section is not in the
provided pages, do not add a physical_index to it. Directly return the final JSON structure. Do not
output anything else.

Table of contents (JSON):
%s

Document pages:
%s`, tocJSON, taggedPages)
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
	return fmt.Sprintf(`You are navigating a document to answer a question. You are given its hierarchical
structure — section titles, summaries, and page ranges (no full text). Use the summaries to judge
what each section actually covers.

Choose the TIGHTEST set of page ranges likely to contain the answer, and include ALL of them:
- For a specific fact, figure, or definition, pick the exact pages of the section or table that
  holds it — not an entire top-level section.
- For "why / how / what caused" questions, include BOTH the explanation AND the supporting detail
  (e.g. the relevant data, table, or example).
- When unsure between two candidate sections, include both — a few extra pages is far cheaper
  than missing the answer.

Document structure:
%s

Question: %s

Reply JSON:
{ "thinking": <which sections are relevant and why, citing their summaries>, "pages": "<page ranges like 5-7,12>" }
Directly return the JSON. Do not output anything else.`, structure, question)
}

// AskSelectMore asks for a DIFFERENT set of pages after the first selection failed
// to yield the answer (the fetch-more step for higher effort levels).
func AskSelectMore(structure, question, triedPages string) string {
	return fmt.Sprintf(`You are navigating a document. You already examined pages %s and could NOT find the
answer there. Pick a DIFFERENT, broader set of page ranges that might contain it — do not repeat
those pages. Consider related sections, tables, appendices, or notes you may have skipped. Use the
section summaries.

Document structure:
%s

Question: %s

Reply JSON:
{ "thinking": <where else the answer might be and why>, "pages": "<new page ranges like 8-10,14>" }
Directly return the JSON. Do not output anything else.`, triedPages, structure, question)
}

// Equivalence is the reply schema for JudgeEquivalence.
type Equivalence struct {
	Thinking string `json:"thinking"`
	Correct  bool   `json:"correct"`
}

// JudgeEquivalence grades a predicted answer against a gold answer with the
// permissive equivalence rubric PageIndex's Mafin 2.5 FinanceBench eval uses
// (rounding/format/superset tolerated), for apples-to-apples comparability.
func JudgeEquivalence(question, gold, predicted string) string {
	return fmt.Sprintf(`You are grading a financial question-answering response.
Mark it CORRECT if the golden answer (or any equivalent of it) can be inferred from
the AI-generated answer. Apply these rules:
- Ignore differences from rounding (e.g. "11 of 14" == "79%%").
- Fractions, percentages, and numerics that are similar count as equivalent
  (e.g. "$1.2B" == "1,200 million").
- An AI answer that contains MORE detail than the golden answer is still CORRECT
  as long as it conveys the golden answer.
- Judge meaning/conclusion, not wording.

Question: %s
Golden answer: %s
AI-generated answer: %s

Reply JSON: { "thinking": <brief reasoning>, "correct": true or false }
Directly return the JSON. Do not output anything else.`, question, gold, predicted)
}

// AskAnswer asks the model to answer strictly from the fetched page content and
// cite the pages it used.
func AskAnswer(question, pagesJSON string) string {
	return fmt.Sprintf(`You are a document QA assistant. Answer the question using ONLY the provided page content.
Work carefully in "thinking":

1. Locate the exact passages, figures, or values on the pages that bear on the question.
2. If the question requires computation or derivation (a number, percentage, rate, ratio, sum,
   difference, or comparison), work it out step by step from the values on the pages and show the
   steps. The exact answer is often NOT stated verbatim — derive it.
3. If several factors, reasons, or items are relevant, identify the PRIMARY one and lead with it.
4. Give a concise, direct "answer" (include units where applicable).
5. Only say you cannot find it if the pages genuinely lack the information needed — and if so, say
   briefly in "thinking" what was missing. Never guess or fabricate; an honest "cannot find it" is
   better than a made-up answer.

Question: %s

Pages (JSON list of {page, content}):
%s

Reply JSON:
{ "thinking": <locate the relevant content and show any derivation step by step>, "answer": <concise, direct answer>, "pages_used": "<page numbers like 5,7>" }
Directly return the JSON. Do not output anything else.`, question, pagesJSON)
}
