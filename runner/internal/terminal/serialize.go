package terminal

import (
	"fmt"
	"strings"
)

// SerializeRange specifies a range of lines to serialize
// Matches xterm.js ISerializeRange interface
type SerializeRange struct {
	// Start line (0-indexed, relative to start of history + screen buffer)
	Start int
	// End line (0-indexed, inclusive, relative to start of history + screen buffer)
	End int
}

// SerializeOptions configures how terminal state is serialized
type SerializeOptions struct {
	// ScrollbackLines limits how many scrollback lines to include.
	// 0 means include all available scrollback.
	ScrollbackLines int
	// ExcludeAltBuffer if true, excludes the alternate screen buffer content.
	ExcludeAltBuffer bool
	// ExcludeModes if true, excludes terminal mode settings.
	ExcludeModes bool
	// Range specifies a specific range of lines to serialize.
	// If set, ScrollbackLines is ignored.
	Range *SerializeRange
}

// DefaultSerializeOptions returns sensible defaults for serialization
func DefaultSerializeOptions() SerializeOptions {
	return SerializeOptions{
		ScrollbackLines:  1000,
		ExcludeAltBuffer: false,
		ExcludeModes:     true,
	}
}

// StringSerializeHandler implements xterm.js-compatible terminal serialization.
// This is a faithful port of xterm.js addon-serialize's StringSerializeHandler.
type StringSerializeHandler struct {
	vt *VirtualTerminal

	// Row tracking
	allRows          []string
	allRowSeparators []string
	currentRow       strings.Builder
	rowIndex         int

	// Null cell optimization (use cursor movement instead of spaces)
	nullCellCount int

	// Current cursor style (for style diffing)
	cursorStyle    Cell
	cursorStyleRow int
	cursorStyleCol int

	// Background cell for BCE (Background Color Erase)
	backgroundCell Cell

	// Position tracking
	firstRow              int
	lastCursorRow         int
	lastCursorCol         int
	lastContentCursorRow  int
	lastContentCursorCol  int
}

// newStringSerializeHandler creates a new handler
func newStringSerializeHandler(vt *VirtualTerminal) *StringSerializeHandler {
	return &StringSerializeHandler{
		vt:             vt,
		cursorStyle:    NewCell(' '),
		backgroundCell: NewCell(' '),
	}
}

// serialize serializes the terminal content from startRow to endRow (inclusive)
func (h *StringSerializeHandler) serialize(startRow, endRow int, excludeFinalCursorPosition bool) string {
	rowCount := endRow - startRow + 1
	h.allRows = make([]string, rowCount)
	h.allRowSeparators = make([]string, rowCount)
	h.firstRow = startRow
	h.lastContentCursorRow = startRow
	h.lastCursorRow = startRow
	h.lastCursorCol = 0
	h.lastContentCursorCol = 0
	h.rowIndex = 0

	// Process each row
	var prevCell Cell = NewCell(' ')
	for row := startRow; row <= endRow; row++ {
		// Reset per-row state
		h.currentRow.Reset()
		h.nullCellCount = 0

		cells := h.vt.GetCellsRow(row)
		if cells != nil {
			for col := 0; col < len(cells); col++ {
				cell := cells[col]
				h.nextCell(cell, prevCell, row, col)
				prevCell = cell
			}
		}
		h.rowEnd(row, row == endRow)
	}

	return h.serializeString(startRow, endRow, excludeFinalCursorPosition)
}

// nextCell processes a single cell
func (h *StringSerializeHandler) nextCell(cell, oldCell Cell, row, col int) {
	// Width 0 cells are placeholders after CJK characters - skip them
	if cell.GetWidth() == 0 {
		return
	}

	// Check if this cell is empty (no character content)
	isEmptyCell := cell.Char == ' ' || cell.Char == 0

	// Calculate style difference
	sgrSeq := h.diffStyle(cell, h.cursorStyle)

	// For empty cells, only background matters for style changes
	styleChanged := false
	if isEmptyCell {
		styleChanged = !cell.Bg.Equals(h.cursorStyle.Bg)
	} else {
		styleChanged = len(sgrSeq) > 0
	}

	// Handle style change
	if styleChanged {
		// Before updating style, handle pending null cells
		if h.nullCellCount > 0 {
			// Use ECH (Erase Character) to set background if needed
			if !h.cursorStyle.Bg.Equals(h.backgroundCell.Bg) {
				h.currentRow.WriteString(fmt.Sprintf("\x1b[%dX", h.nullCellCount))
			}
			// Use CUF (Cursor Forward) to move past null cells
			h.currentRow.WriteString(fmt.Sprintf("\x1b[%dC", h.nullCellCount))
			h.nullCellCount = 0
		}

		h.lastContentCursorRow = row
		h.lastContentCursorCol = col
		h.lastCursorRow = row
		h.lastCursorCol = col

		// Output SGR sequence
		if len(sgrSeq) > 0 {
			h.currentRow.WriteString("\x1b[")
			h.currentRow.WriteString(strings.Join(sgrSeq, ";"))
			h.currentRow.WriteString("m")
		}

		// Update cursor style
		h.cursorStyle = cell
		h.cursorStyleRow = row
		h.cursorStyleCol = col
	}

	// Handle actual content
	if isEmptyCell {
		width := cell.GetWidth()
		if width == 0 {
			width = 1
		}
		h.nullCellCount += int(width)
	} else {
		if h.nullCellCount > 0 {
			// Handle pending null cells
			if h.cursorStyle.Bg.Equals(h.backgroundCell.Bg) {
				// Background is default, just move cursor
				h.currentRow.WriteString(fmt.Sprintf("\x1b[%dC", h.nullCellCount))
			} else {
				// Background has color, use ECH then move
				h.currentRow.WriteString(fmt.Sprintf("\x1b[%dX", h.nullCellCount))
				h.currentRow.WriteString(fmt.Sprintf("\x1b[%dC", h.nullCellCount))
			}
			h.nullCellCount = 0
		}

		// Output the character
		if cell.Char != 0 {
			h.currentRow.WriteRune(cell.Char)
		} else {
			h.currentRow.WriteRune(' ')
		}

		// Update cursor position
		width := cell.GetWidth()
		if width == 0 {
			width = 1
		}
		h.lastContentCursorRow = row
		h.lastContentCursorCol = col + int(width)
		h.lastCursorRow = row
		h.lastCursorCol = col + int(width)
	}
}

// rowEnd handles end of row processing
func (h *StringSerializeHandler) rowEnd(row int, isLastRow bool) {
	// If there are colorful empty cells at line end, we must preserve them
	if h.nullCellCount > 0 && !h.cursorStyle.Bg.Equals(h.backgroundCell.Bg) {
		h.currentRow.WriteString(fmt.Sprintf("\x1b[%dX", h.nullCellCount))
	}

	rowSeparator := ""

	if !isLastRow {
		// Check if next line is wrapped
		if !h.vt.IsLineWrapped(row + 1) {
			// Not wrapped - insert CRLF
			rowSeparator = "\r\n"
			h.lastCursorRow = row + 1
			h.lastCursorCol = 0
		} else {
			// Line is wrapped - no separator needed
			// The content naturally flows to next line
			rowSeparator = ""

			// Update content cursor position for wrapped lines
			h.lastContentCursorRow = row + 1
			h.lastContentCursorCol = 0
			h.lastCursorRow = row + 1
			h.lastCursorCol = 0
		}
	}

	h.allRows[h.rowIndex] = h.currentRow.String()
	h.allRowSeparators[h.rowIndex] = rowSeparator
	h.rowIndex++
	h.currentRow.Reset()
	h.nullCellCount = 0
}

// serializeString builds the final serialized string
func (h *StringSerializeHandler) serializeString(startRow, endRow int, excludeFinalCursorPosition bool) string {
	var content strings.Builder

	// Calculate how many rows to include
	rowEnd := len(h.allRows)

	// If buffer is within screen size, trim trailing empty rows
	bufferLength := endRow - startRow + 1
	if bufferLength <= h.vt.Rows() {
		rowEnd = h.lastContentCursorRow + 1 - h.firstRow
		if rowEnd < 0 {
			rowEnd = 0
		}
		if rowEnd > len(h.allRows) {
			rowEnd = len(h.allRows)
		}
		h.lastCursorCol = h.lastContentCursorCol
		h.lastCursorRow = h.lastContentCursorRow
	}

	// Build content
	for i := 0; i < rowEnd; i++ {
		content.WriteString(h.allRows[i])
		if i+1 < rowEnd {
			content.WriteString(h.allRowSeparators[i])
		}
	}

	// Restore cursor position if needed
	// Use absolute positioning (CUP) for reliable cursor placement
	if !excludeFinalCursorPosition {
		cursorRow, cursorCol := h.vt.CursorPosition()
		// Use CUP (Cursor Position) - 1-indexed
		content.WriteString(fmt.Sprintf("\x1b[%d;%dH", cursorRow+1, cursorCol+1))
	}

	// Restore cursor's current style (important for correct rendering)
	curFg, curBg, curAttrs, curUlStyle, curUlColor := h.vt.GetCurrentStyle()
	curCell := NewFullStyledCell(' ', curFg, curBg, curAttrs, 1, curUlStyle, curUlColor)
	sgrSeq := h.diffStyle(curCell, h.cursorStyle)
	if len(sgrSeq) > 0 {
		content.WriteString("\x1b[")
		content.WriteString(strings.Join(sgrSeq, ";"))
		content.WriteString("m")
	}

	return content.String()
}

// diffStyle generates SGR parameters for style transition
// This is a faithful port of xterm.js StringSerializeHandler._diffStyle
func (h *StringSerializeHandler) diffStyle(cell, oldCell Cell) []string {
	var sgrSeq []string

	fgChanged := !cell.Fg.Equals(oldCell.Fg)
	bgChanged := !cell.Bg.Equals(oldCell.Bg)
	flagsChanged := !equalFlags(cell, oldCell)

	if !fgChanged && !bgChanged && !flagsChanged {
		return nil
	}

	if cell.IsAttributeDefault() {
		if !oldCell.IsAttributeDefault() {
			sgrSeq = append(sgrSeq, "0")
		}
	} else {
		// Foreground color
		if fgChanged {
			sgrSeq = append(sgrSeq, buildFgColorSGR(cell.Fg)...)
		}

		// Background color
		if bgChanged {
			sgrSeq = append(sgrSeq, buildBgColorSGR(cell.Bg)...)
		}

		// Flags
		if flagsChanged {
			sgrSeq = append(sgrSeq, buildFlagsSGR(cell, oldCell)...)
		}
	}

	return sgrSeq
}

// equalFlags compares cell flags for equality
func equalFlags(cell, oldCell Cell) bool {
	if cell.Attrs != oldCell.Attrs {
		return false
	}
	// If underline is set, also check underline style and color
	if cell.Attrs.Has(AttrUnderline) {
		if cell.UnderlineStyle != oldCell.UnderlineStyle {
			return false
		}
		if !cell.UnderlineColor.Equals(oldCell.UnderlineColor) {
			return false
		}
	}
	return true
}

// buildFgColorSGR builds SGR parameters for foreground color
func buildFgColorSGR(c Color) []string {
	if c.IsDefault() {
		return []string{"39"}
	}
	if c.IsPalette() {
		idx := c.Index()
		if idx < 8 {
			return []string{fmt.Sprintf("%d", 30+idx)}
		} else if idx < 16 {
			return []string{fmt.Sprintf("%d", 90+idx-8)}
		}
		return []string{"38", "5", fmt.Sprintf("%d", idx)}
	}
	if c.IsRGB() {
		r, g, b := c.RGB()
		return []string{"38", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b)}
	}
	return nil
}

// buildBgColorSGR builds SGR parameters for background color
func buildBgColorSGR(c Color) []string {
	if c.IsDefault() {
		return []string{"49"}
	}
	if c.IsPalette() {
		idx := c.Index()
		if idx < 8 {
			return []string{fmt.Sprintf("%d", 40+idx)}
		} else if idx < 16 {
			return []string{fmt.Sprintf("%d", 100+idx-8)}
		}
		return []string{"48", "5", fmt.Sprintf("%d", idx)}
	}
	if c.IsRGB() {
		r, g, b := c.RGB()
		return []string{"48", "2", fmt.Sprintf("%d", r), fmt.Sprintf("%d", g), fmt.Sprintf("%d", b)}
	}
	return nil
}

// buildFlagsSGR builds SGR parameters for text attribute changes
func buildFlagsSGR(cell, oldCell Cell) []string {
	var sgrSeq []string

	// Inverse
	if cell.Attrs.Has(AttrInverse) != oldCell.Attrs.Has(AttrInverse) {
		if cell.Attrs.Has(AttrInverse) {
			sgrSeq = append(sgrSeq, "7")
		} else {
			sgrSeq = append(sgrSeq, "27")
		}
	}

	// Bold
	if cell.Attrs.Has(AttrBold) != oldCell.Attrs.Has(AttrBold) {
		if cell.Attrs.Has(AttrBold) {
			sgrSeq = append(sgrSeq, "1")
		} else {
			sgrSeq = append(sgrSeq, "22")
		}
	}

	// Underline (with style support)
	underlineChanged := false
	if cell.Attrs.Has(AttrUnderline) != oldCell.Attrs.Has(AttrUnderline) {
		underlineChanged = true
	} else if cell.Attrs.Has(AttrUnderline) && oldCell.Attrs.Has(AttrUnderline) {
		if cell.UnderlineStyle != oldCell.UnderlineStyle ||
			!cell.UnderlineColor.Equals(oldCell.UnderlineColor) {
			underlineChanged = true
		}
	}

	if underlineChanged {
		if !cell.Attrs.Has(AttrUnderline) {
			sgrSeq = append(sgrSeq, "24")
		} else {
			// Use 4:X format for underline style
			style := cell.UnderlineStyle
			if style == UnderlineNone {
				style = UnderlineSingle
			}
			sgrSeq = append(sgrSeq, fmt.Sprintf("4:%d", style))

			// Add underline color if set
			if !cell.UnderlineColor.IsDefault() {
				if cell.UnderlineColor.IsRGB() {
					r, g, b := cell.UnderlineColor.RGB()
					sgrSeq = append(sgrSeq, fmt.Sprintf("58:2::%d:%d:%d", r, g, b))
				} else if cell.UnderlineColor.IsPalette() {
					sgrSeq = append(sgrSeq, fmt.Sprintf("58:5:%d", cell.UnderlineColor.Index()))
				}
			}
		}
	}

	// Overline
	if cell.Attrs.Has(AttrOverline) != oldCell.Attrs.Has(AttrOverline) {
		if cell.Attrs.Has(AttrOverline) {
			sgrSeq = append(sgrSeq, "53")
		} else {
			sgrSeq = append(sgrSeq, "55")
		}
	}

	// Blink
	if cell.Attrs.Has(AttrBlink) != oldCell.Attrs.Has(AttrBlink) {
		if cell.Attrs.Has(AttrBlink) {
			sgrSeq = append(sgrSeq, "5")
		} else {
			sgrSeq = append(sgrSeq, "25")
		}
	}

	// Invisible
	if cell.Attrs.Has(AttrInvisible) != oldCell.Attrs.Has(AttrInvisible) {
		if cell.Attrs.Has(AttrInvisible) {
			sgrSeq = append(sgrSeq, "8")
		} else {
			sgrSeq = append(sgrSeq, "28")
		}
	}

	// Italic
	if cell.Attrs.Has(AttrItalic) != oldCell.Attrs.Has(AttrItalic) {
		if cell.Attrs.Has(AttrItalic) {
			sgrSeq = append(sgrSeq, "3")
		} else {
			sgrSeq = append(sgrSeq, "23")
		}
	}

	// Dim
	if cell.Attrs.Has(AttrDim) != oldCell.Attrs.Has(AttrDim) {
		if cell.Attrs.Has(AttrDim) {
			sgrSeq = append(sgrSeq, "2")
		} else {
			sgrSeq = append(sgrSeq, "22")
		}
	}

	// Strikethrough
	if cell.Attrs.Has(AttrStrikethrough) != oldCell.Attrs.Has(AttrStrikethrough) {
		if cell.Attrs.Has(AttrStrikethrough) {
			sgrSeq = append(sgrSeq, "9")
		} else {
			sgrSeq = append(sgrSeq, "29")
		}
	}

	return sgrSeq
}

// Serialize returns the terminal state as an ANSI sequence string.
// The output can be written to xterm.js via term.write() to restore the terminal state.
//
// This implementation is a faithful port of xterm.js addon-serialize's StringSerializeHandler.
func (vt *VirtualTerminal) Serialize(opts SerializeOptions) string {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if !vt.hasData {
		return ""
	}

	// Calculate effective range based on options
	historyLen := len(vt.historyStyled)
	totalRows := historyLen + vt.rows

	var startRow, endRow int

	if opts.Range != nil {
		// Use explicit range
		startRow = opts.Range.Start
		endRow = opts.Range.End
		if startRow < 0 {
			startRow = 0
		}
		if endRow >= totalRows {
			endRow = totalRows - 1
		}
	} else {
		// Use scrollback limit
		if opts.ScrollbackLines > 0 && historyLen > opts.ScrollbackLines {
			startRow = historyLen - opts.ScrollbackLines
		} else {
			startRow = 0
		}
		endRow = totalRows - 1
	}

	if startRow > endRow {
		return ""
	}

	// Use the combined serializer that handles both history and screen
	handler := newStringSerializeHandler(vt)
	return handler.serializeWithHistory(startRow, endRow, historyLen, false)
}

// serializeNoLock is like Serialize but assumes the lock is already held.
// Used by GetSnapshot which already holds the read lock.
func (vt *VirtualTerminal) serializeNoLock(opts SerializeOptions) string {
	if !vt.hasData {
		return ""
	}

	// Calculate effective range based on options
	historyLen := len(vt.historyStyled)
	totalRows := historyLen + vt.rows

	var startRow, endRow int

	if opts.Range != nil {
		// Use explicit range
		startRow = opts.Range.Start
		endRow = opts.Range.End
		if startRow < 0 {
			startRow = 0
		}
		if endRow >= totalRows {
			endRow = totalRows - 1
		}
	} else {
		// Use scrollback limit
		if opts.ScrollbackLines > 0 && historyLen > opts.ScrollbackLines {
			startRow = historyLen - opts.ScrollbackLines
		} else {
			startRow = 0
		}
		endRow = totalRows - 1
	}

	if startRow > endRow {
		return ""
	}

	// Use the combined serializer that handles both history and screen
	handler := newStringSerializeHandler(vt)
	return handler.serializeWithHistory(startRow, endRow, historyLen, false)
}

// serializeWithHistory serializes both history and screen content with full style support
// startRow and endRow are in absolute coordinates (0 = first history line)
// historyLen is the number of history lines
func (h *StringSerializeHandler) serializeWithHistory(startRow, endRow, historyLen int, excludeFinalCursorPosition bool) string {
	rowCount := endRow - startRow + 1
	h.allRows = make([]string, rowCount)
	h.allRowSeparators = make([]string, rowCount)
	h.firstRow = startRow
	h.lastContentCursorRow = startRow
	h.lastCursorRow = startRow
	h.lastCursorCol = 0
	h.lastContentCursorCol = 0
	h.rowIndex = 0

	// Process each row (history + screen)
	var prevCell Cell = NewCell(' ')
	for row := startRow; row <= endRow; row++ {
		// Reset per-row state
		h.currentRow.Reset()
		h.nullCellCount = 0

		var cells []Cell
		var isWrapped bool

		if row < historyLen {
			// This row is from styled history
			cells = h.vt.historyStyled[row]
			isWrapped = h.vt.historyIsWrapped[row]
		} else {
			// This row is from current screen
			screenRow := row - historyLen
			if screenRow >= 0 && screenRow < h.vt.rows {
				cells = h.vt.cells[screenRow]
				isWrapped = h.vt.isWrapped[screenRow]
			}
		}

		if cells != nil {
			for col := 0; col < len(cells); col++ {
				cell := cells[col]
				h.nextCell(cell, prevCell, row, col)
				prevCell = cell
			}
		}
		h.rowEndWithWrap(row, row == endRow, isWrapped, row+1 < historyLen || (row+1 >= historyLen && row+1-historyLen < h.vt.rows))
	}

	return h.serializeStringForHistory(startRow, endRow, historyLen, excludeFinalCursorPosition)
}

// rowEndWithWrap handles end of row processing with explicit wrap info
func (h *StringSerializeHandler) rowEndWithWrap(row int, isLastRow bool, currentWrapped bool, hasNextRow bool) {
	// If there are colorful empty cells at line end, we must preserve them
	if h.nullCellCount > 0 && !h.cursorStyle.Bg.Equals(h.backgroundCell.Bg) {
		h.currentRow.WriteString(fmt.Sprintf("\x1b[%dX", h.nullCellCount))
	}

	rowSeparator := ""

	if !isLastRow && hasNextRow {
		// Determine if next line is wrapped
		// For history->screen transition, check screen wrap flag at row 0
		historyLen := len(h.vt.historyStyled)
		var nextLineWrapped bool
		if row+1 < historyLen {
			nextLineWrapped = h.vt.historyIsWrapped[row+1]
		} else {
			screenRow := row + 1 - historyLen
			if screenRow >= 0 && screenRow < h.vt.rows {
				nextLineWrapped = h.vt.isWrapped[screenRow]
			}
		}

		if !nextLineWrapped {
			// Not wrapped - insert CRLF
			rowSeparator = "\r\n"
			h.lastCursorRow = row + 1
			h.lastCursorCol = 0
		} else {
			// Line is wrapped - no separator needed
			rowSeparator = ""
			h.lastContentCursorRow = row + 1
			h.lastContentCursorCol = 0
			h.lastCursorRow = row + 1
			h.lastCursorCol = 0
		}
	}

	h.allRows[h.rowIndex] = h.currentRow.String()
	h.allRowSeparators[h.rowIndex] = rowSeparator
	h.rowIndex++
	h.currentRow.Reset()
	h.nullCellCount = 0
}

// serializeStringForHistory builds the final serialized string for history+screen
func (h *StringSerializeHandler) serializeStringForHistory(startRow, endRow, historyLen int, excludeFinalCursorPosition bool) string {
	var content strings.Builder

	// Calculate how many rows to include
	rowEnd := len(h.allRows)

	// If buffer is within screen size, trim trailing empty rows
	bufferLength := endRow - startRow + 1
	if bufferLength <= h.vt.Rows() {
		rowEnd = h.lastContentCursorRow + 1 - h.firstRow
		if rowEnd < 0 {
			rowEnd = 0
		}
		if rowEnd > len(h.allRows) {
			rowEnd = len(h.allRows)
		}
		h.lastCursorCol = h.lastContentCursorCol
		h.lastCursorRow = h.lastContentCursorRow
	}

	// Build content
	for i := 0; i < rowEnd; i++ {
		content.WriteString(h.allRows[i])
		if i+1 < rowEnd {
			content.WriteString(h.allRowSeparators[i])
		}
	}

	// Restore cursor position if needed
	// Use absolute positioning (CUP) for screen-relative cursor position
	// This is important when history is included, as relative moves won't work correctly
	// after xterm.js auto-scrolls to the bottom
	if !excludeFinalCursorPosition {
		// Cursor position relative to visible screen (1-indexed for CUP command)
		cursorRow := h.vt.cursorY + 1 // Convert to 1-indexed
		cursorCol := h.vt.cursorX + 1 // Convert to 1-indexed

		// Use CUP (Cursor Position) for absolute positioning within visible screen
		content.WriteString(fmt.Sprintf("\x1b[%d;%dH", cursorRow, cursorCol))
	}

	// Restore cursor's current style (important for correct rendering)
	curFg, curBg, curAttrs, curUlStyle, curUlColor := h.vt.getCurrentStyleNoLock()
	curCell := NewFullStyledCell(' ', curFg, curBg, curAttrs, 1, curUlStyle, curUlColor)
	sgrSeq := h.diffStyle(curCell, h.cursorStyle)
	if len(sgrSeq) > 0 {
		content.WriteString("\x1b[")
		content.WriteString(strings.Join(sgrSeq, ";"))
		content.WriteString("m")
	}

	return content.String()
}

// SerializeSimple returns a simple serialization without style information.
// This is faster and suitable for agent observation where colors don't matter.
func (vt *VirtualTerminal) SerializeSimple(scrollbackLines int) string {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if !vt.hasData {
		return ""
	}

	var buf strings.Builder

	// Include history
	historyStart := 0
	if scrollbackLines > 0 && len(vt.history) > scrollbackLines {
		historyStart = len(vt.history) - scrollbackLines
	}

	for i := historyStart; i < len(vt.history); i++ {
		buf.WriteString(vt.history[i])
		buf.WriteString("\r\n")
	}

	// Include screen (using existing rune-based buffer for backward compatibility)
	for row := 0; row < vt.rows; row++ {
		line := strings.TrimRight(string(vt.screen[row]), " ")
		buf.WriteString(line)
		if row < vt.rows-1 {
			buf.WriteString("\r\n")
		}
	}

	return buf.String()
}
