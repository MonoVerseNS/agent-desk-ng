package ai

import (
	"testing"
)

func TestIsCustomerHandoffRequest(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Russian varied phrasing (not just "переведи на оператора")
		{"переведи на оператора", true},
		{"переведите на оператора пожалуйста", true},
		{"соедините со специалистом", true},
		{"позови сотрудника", true},
		{"позовите сотрудника поддержки", true},
		{"мне нужен живой человек", true},
		{"хочу поговорить с человеком", true},
		{"хочу поговорить со специалистом", true},
		{"хватит бота", true},
		{"хватит робота, дайте человека", true},
		{"не хочу общаться с ботом", true},
		{"можно человека?", true},
		{"есть кто живой?", true},
		{"свяжите с оператором", true},
		{"переключи на оператора", true},
		{"дай оператора", true},
		{"дай человека", true},
		{"перенаправь оператору", true},
		{"перенаправьте на специалиста", true},
		{"тупой бот, дай человека", true},
		{"нужен оператор", true},
		{"нужен специалист", true},
		{"оператор", true},
		{"человек", true},
		{"к оператору", true},

		// English varied phrasing
		{"can I speak to someone?", true},
		{"connect me to an agent", true},
		{"I want to talk to a real person", true},
		{"let me speak with support", true},
		{"transfer me to a representative", true},
		{"representative please", true},
		{"human please", true},
		{"stop bot and get human", true},
		{"live agent", true},
		{"operator", true},
		{"human", true},

		// Other languages
		{"necesito hablar con una persona", true},
		{"quiero hablar con un humano", true},
		{"mit einem Mitarbeiter sprechen", true},
		{"转人工", true},
		{"请帮我转接人工客服", true},

		// Non-handoff questions & greetings (including mentions of operators without transfer intent)
		{"Привет хочу возрат денег", false},
		{"Здравствуйте, как вернуть товар?", false},
		{"Добрый день, сколько стоит доставка?", false},
		{"Hello, how do I track my order?", false},
		{"Can I get a refund for my purchase?", false},
		{"Операторы не создаются", false},
		{"Операторы не создаються", false},
		{"Кто такой оператор?", false},
		{"Оператор работает ночью?", false},
		{"Спасибо, всё понятно!", false},
		{"OK, thank you very much", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsCustomerHandoffRequest(tt.input)
			if got != tt.expected {
				t.Errorf("IsCustomerHandoffRequest(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsModelClaimingHandoff(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Russian model claims
		{"Похоже я не могу решить ваш вопрос, перенаправляю оператору. Ожидайте!", true},
		{"Перевожу вас на оператора, пожалуйста подождите.", true},
		{"Переключаю вас на оператора службы поддержки.", true},
		{"Перенаправляю вас на специалиста.", true},
		{"Соединяю со специалистом, оставайтесь на связи.", true},
		{"Передаю ваш запрос оператору.", true},
		{"Передаю диалог коллегам из поддержки.", true},
		{"Сейчас переведу на оператора.", true},

		// English model claims
		{"Transferring you to an agent, please wait a moment.", true},
		{"Connecting you to an operator right away.", true},
		{"Connecting you to a human representative.", true},
		{"Routing you to an agent.", true},
		{"I am passing you to our support team.", true},

		// Chinese model claims
		{"正在为您转接人工客服，请稍候。", true},
		{"为您转接人工客服处理。", true},

		// Normal replies
		{"Здравствуйте! Чем я могу вам помочь?", false},
		{"Чтобы оформить возврат, перейдите в личный кабинет.", false},
		{"To return an item, please open your order history.", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsModelClaimingHandoff(tt.input)
			if got != tt.expected {
				t.Errorf("IsModelClaimingHandoff(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
