package app

import "strings"

const (
	languageEnglish = "en"
	languageRussian = "ru"
)

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	switch {
	case language == "", strings.HasPrefix(language, "en"):
		return languageEnglish
	case strings.HasPrefix(language, "ru"):
		return languageRussian
	default:
		return languageEnglish
	}
}

func (m Model) language() string {
	return normalizeLanguage(m.cfg.Language)
}

func (m Model) isRussian() bool {
	return m.language() == languageRussian
}

func translate(language, key string) string {
	if normalizeLanguage(language) != languageRussian {
		return key
	}
	if v, ok := ruStrings[key]; ok {
		return v
	}
	return key
}

func (m Model) tr(key string) string {
	return translate(m.cfg.Language, key)
}

var ruStrings = map[string]string{
	"current channel":                "текущий канал",
	"to":                             "в",
	"reply to":                       "ответ для",
	"Write a message…":               "Написать сообщение…",
	"Write a reply…":                 "Написать ответ…",
	"enter send":                     "enter отправить",
	"alt+enter newline":              "alt+enter новая строка",
	"composer inactive":              "поле ввода не активно",
	"tab focus input":                "tab к вводу",
	"reply composer inactive":        "ответ не активен",
	"tab reply":                      "tab ответ",
	"reply in thread":                "ответ в тред",
	"at latest":                      "последние сообщения",
	"scrolled":                       "прокручено",
	"ready":                          "готово",
	"net":                            "сеть",
	"scope":                          "область",
	"cached · connecting…":           "кэш · подключение…",
	"cached · refreshing scope…":     "кэш · обновляю область…",
	"connecting…":                    "подключение…",
	"connected":                      "подключено",
	"reconnecting…":                  "переподключение…",
	"offline":                        "нет соединения",
	"auth expired":                   "сессия истекла",
	"refresh token and restart":      "обновите токен и перезапустите",
	"Loading messages…":              "Загружаю сообщения…",
	"Loading thread…":                "Загружаю тред…",
	"No messages yet.":               "Сообщений пока нет.",
	"No replies yet.":                "Ответов пока нет.",
	"Loading older messages…":        "Загружаю старые сообщения…",
	"Beginning of history":           "Начало истории",
	"Press [ to load older messages": "Нажмите [ чтобы загрузить старые сообщения",
	"new messages":                   "новые сообщения",
	"unread selected":                "выбрано непрочитанное",
	"today":                          "сегодня",
	"yesterday":                      "вчера",
	"messages":                       "сообщений",
	"message":                        "сообщение",
	"file":                           "файл",
	"reply":                          "ответ",
	"replies":                        "ответов",
	"new replies":                    "новые ответы",
	"sidebar":                        "сайдбар",
	"timeline":                       "лента",
	"composer":                       "ввод",
	"thread":                         "тред",
	"Thread":                         "Тред",
	"thread reply":                   "ответ в тред",
	"thread messages":                "сообщения треда",
	"filter channels":                "фильтр каналов",
	"Settings":                       "Настройки",
	"Server URL":                     "Адрес сервера",
	"Token":                          "Токен",
	"Save connection":                "Сохранить подключение",
	"save to config":                 "сохранить в config",
	"not set":                        "не задано",
	"move":                           "двигаться",
	"edit/save":                      "редактировать/сохранить",
	"editing: type, enter save, esc cancel, ctrl+u clear": "редактирование: ввод, enter сохранить, esc отмена, ctrl+u очистить",
	"Connection changes are used after restart.":          "Изменения подключения применятся после перезапуска.",
	"Enter server URL and token, save, then restart.":     "Введите адрес сервера и токен, сохраните и перезапустите.",
	"setup required":                                 "нужна настройка",
	"connection settings saved":                      "настройки подключения сохранены",
	"connection settings saved · restart to connect": "настройки подключения сохранены · перезапустите для подключения",
	"server URL is required":                         "адрес сервера обязателен",
	"Language":                                       "Язык",
	"language":                                       "язык",
	"English":                                        "английский",
	"Russian":                                        "русский",
	"change":                                         "изменить",
	"press r for Russian, e for English":             "r — русский, e — английский",
	"Settings are saved to config.":                  "Настройки сохраняются в config.",
	"type command or channel…":                       "команда или канал…",
	"Go to":                                          "Перейти",
	"No matches":                                     "Нет совпадений",
	"Go: Sidebar":                                    "Перейти: сайдбар",
	"Go: Timeline":                                   "Перейти: лента",
	"Go: Composer":                                   "Перейти: ввод",
	"Go: Thread messages":                            "Перейти: сообщения треда",
	"Open: Triage inbox":                             "Открыть: triage",
	"Open: Mentions inbox":                           "Открыть: упоминания",
	"Open: Settings":                                 "Открыть: настройки",
	"Go: unknown":                                    "Перейти: неизвестно",
	"channels":                                       "каналы",
	"favorite":                                       "избранное",
	"direct":                                         "личные",
	"group":                                          "группа",
	"loading…":                                       "загрузка…",
	"loading messages…":                              "загружаю сообщения…",
	"loading thread…":                                "загружаю тред…",
	"loading scope…":                                 "загружаю область…",
	"loading scopes…":                                "загружаю области…",
	"loading channels…":                              "загружаю каналы…",
	"loading all scopes…":                            "загружаю все области…",
	"loading older…":                                 "загружаю старые…",
	"reloading…":                                     "перезагружаю…",
	"reloading scope…":                               "перезагружаю область…",
	"reconnected · refreshing…":                      "соединение восстановлено · обновляю…",
	"refresh failed · showing cached messages":       "обновить не удалось · показываю кэш",
	"thread refresh failed · showing cached replies": "тред не обновился · показываю кэш",
	"could not load messages":                        "не удалось загрузить сообщения",
	"could not load thread":                          "не удалось загрузить тред",
	"could not load older messages":                  "не удалось загрузить старые сообщения",
	"could not load channels":                        "не удалось загрузить каналы",
	"connection failed":                              "не удалось подключиться",
	"network error":                                  "ошибка сети",
	"network timeout":                                "таймаут сети",
	"server error":                                   "ошибка сервера",
	"request failed":                                 "запрос не удался",
	"sending…":                                       "отправляю…",
	"sending reply…":                                 "отправляю ответ…",
	"sent":                                           "отправлено",
	"reply sent":                                     "ответ отправлен",
	"send failed · draft restored":                   "отправка не удалась · черновик восстановлен",
	"reply failed · draft restored":                  "ответ не отправлен · черновик восстановлен",
	"opened from triage · type reply":                "открыто из triage · можно отвечать",
	"opening from triage…":                           "открываю из triage…",
	"opened thread · type reply":                     "тред открыт · можно отвечать",
	"pick a reaction":                                "выберите реакцию",
	"toggling reaction…":                             "обновляю реакцию…",
	"reaction added":                                 "реакция добавлена",
	"reaction removed":                               "реакция удалена",
	"reaction failed":                                "реакция не удалась",
	"new reaction":                                   "новая реакция",
	"Reactions":                                      "Реакции",
	"type search · arrows move · enter toggle · esc close": "печатайте для поиска · стрелки выбор · enter переключить · esc закрыть",
	"filter": "фильтр",
	"filter: all available + reactions on this post": "фильтр: все доступные + реакции на этом сообщении",
	"No matching reactions.":                         "Нет подходящих реакций.",
	"selected":                                       "выбрано",
	"Triage":                                         "Triage",
	"open":                                           "открыть",
	"done":                                           "готово",
	"close":                                          "закрыть",
	"Nothing to triage.":                             "Нет важных сообщений.",
	"triage item dismissed":                          "элемент скрыт",
	"nothing to dismiss":                             "нечего скрывать",
	"mention activity cleared":                       "упоминания очищены",
	"no more unread messages":                        "больше нет непрочитанных",
	"jumped to unread":                               "переход к непрочитанному",
	"no link in selected message":                    "в сообщении нет ссылки",
	"opening link…":                                  "открываю ссылку…",
	"link opened":                                    "ссылка открыта",
	"copying message…":                               "копирую сообщение…",
	"message copied":                                 "сообщение скопировано",
	"copying permalink…":                             "копирую ссылку…",
	"permalink copied":                               "ссылка скопирована",
	"message quoted":                                 "сообщение процитировано",
	"editing message":                                "редактирование сообщения",
	"updating…":                                      "обновляю…",
	"message updated":                                "сообщение обновлено",
	"deleting message…":                              "удаляю…",
	"message deleted":                                "сообщение удалено",
	"press D again to delete":                        "нажмите D ещё раз для удаления",
	"can only edit your own messages":                "можно редактировать только свои сообщения",
	"can only delete your own messages":              "можно удалять только свои сообщения",
	"selected message is empty":                      "выбранное сообщение пустое",
	"filter closed":                                  "фильтр закрыт",
	"filter cleared":                                 "фильтр очищен",
	"no channels in section":                         "в секции нет каналов",
	"added to favorites":                             "добавлено в избранное",
	"removed from favorites":                         "удалено из избранного",
	"no thread open":                                 "тред не открыт",
	"unknown":                                        "неизвестно",
}
