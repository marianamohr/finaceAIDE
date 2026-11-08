package ai

const (
	TaskOCRReceipt        = "ocr_receipt"
	TaskFinancialChat     = "financial_chat"
	TaskReport            = "financial_report"
	TaskPantryReceipt     = "pantry_receipt"
	TaskPantryRecipe      = "pantry_recipe"
	TaskPantryConsumeText = "pantry_consume_text"
)

// SelectModel returns the appropriate Claude model for each task type.
func SelectModel(taskType string) string {
	switch taskType {
	case TaskPantryReceipt:
		return "claude-sonnet-5"
	case TaskPantryRecipe:
		return "claude-sonnet-5"
	case TaskPantryConsumeText:
		return "claude-sonnet-5"
	default:
		return "claude-sonnet-4-6"
	}
}
