package ai

import (
	"strings"
	"unicode"
)

// IsCustomerHandoffRequest determines whether a user's message expresses an intent
// to speak with a human operator, agent, specialist, manager, or live person, across
// varied natural phrasing, colloquialisms, and multiple languages (Russian, English,
// Spanish, German, French, Chinese, etc.).
func IsCustomerHandoffRequest(text string) bool {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)

	// Direct Chinese keywords
	if IsHan(lower) {
		for _, kw := range []string{
			"人工", "转人工", "真人", "人工客服", "接人工", "转接人工", "转坐席", "人工服务", "人工专员",
		} {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}

	// Clean punctuation for token / n-gram checks
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			return ' '
		}
		return r
	}, lower)
	fields := strings.Fields(cleaned)
	if len(fields) == 0 {
		return false
	}
	normalized := " " + strings.Join(fields, " ") + " "

	// Russian handoff patterns & stems:
	// Anti-bot phrases: "хватит бота", "не хочу бота", "отключи бота", "надоел бот"
	if strings.Contains(normalized, " хватит бота ") ||
		strings.Contains(normalized, " хватит робота ") ||
		strings.Contains(normalized, " не хочу бота ") ||
		strings.Contains(normalized, " не хочу общаться с ботом ") ||
		strings.Contains(normalized, " надоел бот ") ||
		strings.Contains(normalized, " отключи бота ") ||
		strings.Contains(normalized, " без бота ") ||
		strings.Contains(normalized, " убрать бота ") ||
		strings.Contains(normalized, " stop bot ") ||
		strings.Contains(normalized, " no bot ") ||
		strings.Contains(normalized, " hate bot ") ||
		strings.Contains(normalized, " not a bot ") {
		return true
	}

	russianTargetStems := []string{
		"оператор", "человек", "специалист", "сотрудник", "консультант",
		"менеджер", "живо", "живой", "поддержк", "саппорт",
	}

	russianActionStems := []string{
		"переведи", "переключ", "соедин", "позов", "дайт", "дай", "свяж", "связать",
		"вызов", "вызовите", "хочу", "нужен", "нужна", "нужно", "можно", "позовите",
		"соедините", "переведите", "переключите", "пригласи", "пригласите",
	}

	// Action + Target combinations in Russian (e.g. "позови сотрудника", "соедини со специалистом", "хочу человека")
	for _, action := range russianActionStems {
		if strings.Contains(normalized, " "+action) {
			for _, target := range russianTargetStems {
				if strings.Contains(normalized, " "+target) {
					return true
				}
			}
		}
	}

	// Direct targets in Russian with prepositions or adjectives (e.g. "к оператору", "с человеком", "со специалистом", "живой человек")
	if strings.Contains(normalized, " оператор") ||
		strings.Contains(normalized, " оператора") ||
		strings.Contains(normalized, " оператором") ||
		strings.Contains(normalized, " живой человек") ||
		strings.Contains(normalized, " живого человека") ||
		strings.Contains(normalized, " живым человеком") ||
		strings.Contains(normalized, " со специалистом") ||
		strings.Contains(normalized, " с человеком") ||
		strings.Contains(normalized, " с сотрудником") ||
		strings.Contains(normalized, " поговорить с кем") ||
		strings.Contains(normalized, " поговорить со ") ||
		strings.Contains(normalized, " реальный человек") ||
		strings.Contains(normalized, " живой есть") ||
		strings.Contains(normalized, " есть живой") ||
		strings.Contains(normalized, " есть кто живой") ||
		strings.Contains(normalized, " можно человека") {
		return true
	}

	// English handoff patterns
	englishTargets := []string{
		"operator", "agent", "human", "representative", "person", "specialist", "someone", "support",
	}
	englishActions := []string{
		"talk to", "speak to", "speak with", "connect", "transfer", "get me", "call", "reach", "need", "want", "let me",
	}

	for _, action := range englishActions {
		if strings.Contains(normalized, " "+action) {
			for _, target := range englishTargets {
				if strings.Contains(normalized, " "+target) {
					return true
				}
			}
		}
	}

	if strings.Contains(normalized, " real person ") ||
		strings.Contains(normalized, " live agent ") ||
		strings.Contains(normalized, " live person ") ||
		strings.Contains(normalized, " human agent ") ||
		strings.Contains(normalized, " human please ") ||
		strings.Contains(normalized, " customer service ") ||
		strings.Contains(normalized, " human support ") ||
		strings.Contains(normalized, " talk to someone ") ||
		strings.Contains(normalized, " speak to someone ") ||
		strings.Contains(normalized, " representative please ") ||
		strings.Contains(normalized, " representative ") {
		return true
	}

	// Spanish handoff patterns
	if strings.Contains(normalized, " hablar con ") ||
		strings.Contains(normalized, " pasar con ") ||
		strings.Contains(normalized, " contactar con ") ||
		strings.Contains(normalized, " quiero hablar con ") ||
		strings.Contains(normalized, " necesito hablar con ") {
		if strings.Contains(normalized, " persona") ||
			strings.Contains(normalized, " humano") ||
			strings.Contains(normalized, " agente") ||
			strings.Contains(normalized, " operador") ||
			strings.Contains(normalized, " alguien") {
			return true
		}
	}
	if strings.Contains(normalized, " atencion al cliente") ||
		strings.Contains(normalized, " persona real") ||
		strings.Contains(normalized, " agente humano") {
		return true
	}

	// German handoff patterns
	if strings.Contains(normalized, " mitarbeiter ") ||
		strings.Contains(normalized, " echter mensch ") ||
		strings.Contains(normalized, " mit jemandem sprechen ") {
		return true
	}

	// French handoff patterns
	if strings.Contains(normalized, " parler a quelqu un ") ||
		strings.Contains(normalized, " conseiller ") ||
		strings.Contains(normalized, " agent humain ") ||
		strings.Contains(normalized, " etre mis en relation ") {
		return true
	}

	return false
}

// IsModelClaimingHandoff checks whether an AI model's text reply claims that it is
// transferring, connecting, or forwarding the conversation to a human operator/agent.
func IsModelClaimingHandoff(text string) bool {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)

	// Russian model handoff statements
	russianModelClaims := []string{
		"перевожу вас на оператора",
		"переключаю вас на оператора",
		"перевожу на оператора",
		"переключаю на оператора",
		"соединяю с оператором",
		"соединяю со специалистом",
		"передаю диалог оператору",
		"передаю ваш запрос оператору",
		"перевожу диалог на оператора",
		"переключаю на специалиста",
		"перевожу на специалиста",
		"соединяю с живым",
		"связываю вас с оператором",
		"сейчас переведу на оператора",
		"сейчас переключу на оператора",
	}
	for _, claim := range russianModelClaims {
		if strings.Contains(lower, claim) {
			return true
		}
	}

	// English model handoff statements
	englishModelClaims := []string{
		"transferring you to an operator",
		"transferring you to an agent",
		"transferring you to a human",
		"transferring you to human",
		"transferring you to our support",
		"connecting you to an operator",
		"connecting you to an agent",
		"connecting you to a human",
		"connecting you with an agent",
		"connecting you with an operator",
		"routing you to an agent",
		"handing you over to",
	}
	for _, claim := range englishModelClaims {
		if strings.Contains(lower, claim) {
			return true
		}
	}

	// Chinese model handoff statements
	chineseModelClaims := []string{
		"为您转接人工",
		"正在为您转接",
		"为您接入人工",
		"转接人工客服",
		"转接人工",
		"已为您转接",
	}
	for _, claim := range chineseModelClaims {
		if strings.Contains(lower, claim) {
			return true
		}
	}

	return false
}
