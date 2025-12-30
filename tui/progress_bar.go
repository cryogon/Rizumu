package main

type ProgressBar struct {
	str string

	Width int

	// what ascii to show in progress ProgressBar
	// -----========
	// where - is AsciiCompleted and = is AsciiNotCompleted
	ASCIICompleted    string
	ASCIINotCompleted string
}

func NewProgressBar() *ProgressBar {
	return &ProgressBar{
		ASCIICompleted:    "░",
		ASCIINotCompleted: "▓",
	}
}

func (p *ProgressBar) View() string {
	return p.str
}

func (p *ProgressBar) Update(width, currentProgress, totalDuration int) {
	p.str = ""

	normalizeProgress := (float64(currentProgress) / float64(totalDuration))
	barWidth := width - 10

	for i := 0; i < barWidth; i++ {
		currentNormalizeProgress := float64(i) / float64(barWidth)
		if currentNormalizeProgress < normalizeProgress {
			p.str += p.ASCIICompleted
		} else {
			p.str += p.ASCIINotCompleted
		}
	}
}
