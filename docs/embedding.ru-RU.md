# Встраивание виджета поддержки (Интеграция)

Виджет онлайн-чата AgentDesk добавляет кнопку вызова поддержки и интерактивный диалог на любую веб-страницу. Он загружается одним скриптом, отображает плавающую кнопку в углу экрана и открывает интерфейс чата внутри изолированного `iframe`. Посетители могут вести несколько обращений параллельно и переключаться между ними.

[English](./embedding.md) | [简体中文](./embedding.zh-CN.md)

---

## Быстрый старт

Замените `https://support.example.com` на URL вашего развёрнутого сервера AgentDesk, а `YOUR_CHANNEL_ID` — на идентификатор канала из панели управления (**Каналы** -> ID нужного канала).

```html
<script>
  window.AgentDeskConfig = {
    channelId: "YOUR_CHANNEL_ID",
  }
</script>
<script src="https://support.example.com/sdk/agent-desk-sdk.min.js" async></script>
```

Разместите оба тега перед закрывающим тегом `</body>`. Блок конфигурации `window.AgentDeskConfig` обязательно должен выполняться **до** загрузки скрипта SDK.

> [!TIP]
> Заголовок, подзаголовок, основной цвет темы, позиция (слева/справа) и ширина виджета по умолчанию подтягиваются из настроек канала в панели управления, поэтому их необязательно прописывать в коде.

---

## Идентификация пользователей (Авторизация клиентов)

По умолчанию любой посетитель считается **анонимным гостем** с браузерным идентификатором вида `guest_xxxxxxxx`. 

Чтобы связать диалог с реальным пользователем вашего сайта, передавать его имя и историю обращений, платформа требует криптографического подтверждения личности. Это защищает от спуфинга: посетитель не может просто указать чужой `userId` в браузере и прочитать чужие конфиденциальные сообщения.

Поддерживаются два способа авторизации: **JWT-токен (`userToken`)** и **HMAC-подпись (`externalId`)**.

---

### Вариант 1: JWT-токен пользователя (`userToken`) — Рекомендуемый

В этом варианте ваш сервер выпускает подписанный JWT-токен при авторизации пользователя на вашем сайте.

#### 1. Настройка секрета в панели управления
1. Откройте панель администратора: **Каналы** -> выберите нужный канал (или создайте новый Web/Chat канал).
2. Найдите поле **User Token Secret** (секретный ключ токена пользователя) и сохраните надёжную случайную строку (например, 32+ символа).

#### 2. Полезная нагрузка токена (JWT Claims)
JWT подписывается алгоритмом `HS256` (также поддерживаются `HS384` и `HS512`) и должен содержать следующие поля:
```json
{
  "userId": "12345",          // Уникальный ID пользователя в вашей системе (string)
  "name": "Иван Иванов",       // Отображаемое имя пользователя (string)
  "exp": 1792100000            // Время истечения срока действия (Unix timestamp в секундах)
}
```

#### 3. Генерация токена на вашем бэкенде

**Node.js / Express**:
```javascript
import jwt from "jsonwebtoken";

app.get("/api/support/user-token", requireAuth, (req, res) => {
  const token = jwt.sign(
    {
      userId: String(req.user.id),
      name: req.user.fullName || req.user.email,
    },
    process.env.AGENT_DESK_CHANNEL_SECRET, // User Token Secret из настроек канала
    {
      algorithm: "HS256",
      expiresIn: "24h",
    }
  );

  res.json({ token });
});
```

**Python (FastAPI / Flask + PyJWT)**:
```python
import time
import jwt

@app.get("/api/support/user-token")
def get_support_user_token(current_user: User = Depends(get_current_user)):
    payload = {
        "userId": str(current_user.id),
        "name": current_user.name,
        "exp": int(time.time()) + 86400, # 24 часа
    }
    token = jwt.encode(payload, os.environ["AGENT_DESK_CHANNEL_SECRET"], algorithm="HS256")
    return {"token": token}
```

**PHP**:
```php
use Firebase\JWT\JWT;

$payload = [
    'userId' => (string)$user->id,
    'name'   => $user->name,
    'exp'    => time() + 86400,
];

$token = JWT::encode($payload, getenv('AGENT_DESK_CHANNEL_SECRET'), 'HS256');
echo json_encode(['token' => $token]);
```

**Go**:
```go
claims := jwt.MapClaims{
    "userId": fmt.Sprintf("%d", user.ID),
    "name":   user.Name,
    "exp":    time.Now().Add(24 * time.Hour).Unix(),
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
signedToken, err := token.SignedString([]byte(channelSecret))
```

#### 4. Передача в виджет на фронтенде
Передайте функцию `getUserToken` в конфигурацию виджета. SDK будет автоматически запрашивать свежий токен при открытии окна чата:
```javascript
window.AgentDeskConfig = {
  channelId: "YOUR_CHANNEL_ID",
  getUserToken: async () => {
    const res = await fetch("/api/support/user-token");
    const data = await res.json();
    return data.token;
  },
};
```

---

### Вариант 2: HMAC-SHA256 подпись (`externalId` + `signExternalId`)

Если вы предпочитаете не выпускать полный JWT, можно передавать ID пользователя и подписывать его через HMAC:

$$\text{signature} = \text{hex}\Big(\text{HMAC-SHA256}\big(\text{secret},\, \text{channelId} + \text{"\textbackslash n"} + \text{externalId} + \text{"\textbackslash n"} + \text{issuedAt}\big)\Big)$$

- `secret`: общий ключ `identity.secret` из файла `config.yaml` (переменная окружения `IDENTITY_HMAC_SECRET`).
- `channelId`: идентификатор канала в AgentDesk.
- `externalId`: ID пользователя на вашем сайте.
- `issuedAt`: текущее время Unix в секундах.

> [!IMPORTANT]
> Подпись должна вычисляться **исключительно на вашем бэкенде**. Никогда не передавайте `identity.secret` в браузер.

**Пример эндпоинта подписи (Node.js)**:
```javascript
import crypto from "node:crypto";

app.get("/api/support/sign", requireAuth, (req, res) => {
  const issuedAt = Math.floor(Date.now() / 1000);
  const channelId = process.env.AGENT_DESK_CHANNEL_ID;
  const externalId = String(req.user.id);

  const payload = `${channelId}\n${externalId}\n${issuedAt}`;
  const signature = crypto
    .createHmac("sha256", process.env.AGENT_DESK_IDENTITY_SECRET)
    .update(payload)
    .digest("hex");

  res.json({ signature, issuedAt });
});
```

**Подключение в виджете**:
```javascript
window.AgentDeskConfig = {
  channelId: "YOUR_CHANNEL_ID",
  externalId: currentUserId,
  externalName: currentUserName,
  signExternalId: async (id) => {
    const res = await fetch(`/api/support/sign?id=${encodeURIComponent(id)}`);
    return await res.json(); // { signature: "...", issuedAt: 171... }
  },
};
```

---

## Примеры для популярных фронтенд-фреймворков

### React / Next.js (App Router)

Поскольку функции `getUserToken` или `signExternalId` нельзя передать в JSON-строке скрипта, для них используется программное монтирование через `AgentDeskWidget.mount`:

```tsx
"use client"

import { useEffect } from "react"

declare global {
  interface Window {
    AgentDeskWidget?: {
      mount: (config: Record<string, unknown>) => void
      destroy: () => void
      open: () => Promise<void>
      close: () => void
    }
  }
}

export function SupportChatWidget({
  channelId,
  user,
}: {
  channelId: string
  user?: { id: string; name: string } | null
}) {
  useEffect(() => {
    const script = document.createElement("script")
    script.src = "https://support.example.com/sdk/agent-desk-sdk.min.js"
    script.async = true
    script.onload = () => {
      window.AgentDeskWidget?.mount({
        channelId,
        language: "ru-RU",
        getUserToken: user
          ? async () => {
              const res = await fetch("/api/support/user-token")
              const data = await res.json()
              return data.token
            }
          : undefined,
      })
    }

    document.body.appendChild(script)

    return () => {
      window.AgentDeskWidget?.destroy()
      script.remove()
    }
  }, [channelId, user?.id])

  return null
}
```

### Vue 3 / Nuxt 3

```vue
<script setup>
import { onMounted, onUnmounted } from "vue"

const props = defineProps({
  channelId: { type: String, required: true },
})

onMounted(() => {
  const script = document.createElement("script")
  script.src = "https://support.example.com/sdk/agent-desk-sdk.min.js"
  script.async = true
  script.onload = () => {
    window.AgentDeskWidget?.mount({
      channelId: props.channelId,
      language: "ru-RU",
      getUserToken: async () => {
        const { data } = await useFetch("/api/support/user-token")
        return data.value?.token
      },
    })
  }
  document.body.appendChild(script)
})

onUnmounted(() => {
  window.AgentDeskWidget?.destroy()
})
</script>
```

---

## Таблица всех параметров конфигурации

| Параметр | Тип | По умолчанию | Описание |
| :--- | :--- | :--- | :--- |
| `channelId` | `string` | — | **Обязательный.** Идентификатор канала из панели управления. |
| `baseUrl` | `string` | домен скрипта | Базовый URL сервера AgentDesk. |
| `apiBaseUrl` | `string` | `baseUrl` | URL для запросов API (если API вынесен на отдельный домен). |
| `widgetBaseUrl` | `string` | домен скрипта | URL, с которого загружается страница фрейма чата. |
| `language` | `"ru-RU" \| "en-US" \| "zh-CN"` | язык сервера | Язык интерфейса виджета чата. |
| `position` | `"left" \| "right"` | из настроек канала | Сторона экрана, где находится кнопка и всплывающее окно. |
| `themeColor` | `string` | из настроек канала | Основной цвет темы (HEX, например `#2563eb`). |
| `width` | `string` | `380px` | Ширина окна чата на экранах десктопа (например `400px`). |
| `title` | `string` | из настроек канала | Заголовок окна поддержки в шапке. |
| `subtitle` | `string` | из настроек канала | Подзаголовок окна поддержки. |
| `externalId` | `string` | браузерный ID | Уникальный ID вашего пользователя. |
| `externalName` | `string` | — | Имя пользователя (используется при создании профиля). |
| `signExternalId` | `(id) => Promise<{signature, issuedAt}>` | — | Функция вычисления HMAC-подписи для `externalId`. |
| `getUserToken` | `() => Promise<string>` | — | Функция получения JWT-токена пользователя. |

---

## JavaScript API виджета

После загрузки SDK в объекте `window.AgentDeskWidget` доступны методы управления:

```javascript
// Открыть окно чата поддержки
await AgentDeskWidget.open();

// Свернуть окно чата
AgentDeskWidget.close();

// Смонтировать виджет с явной конфигурацией
AgentDeskWidget.mount({ channelId: "..." });

// Полностью удалить кнопку и фрейм чата из DOM
AgentDeskWidget.destroy();

// Получить прямую ссылку на чат (для открытия в новой вкладке или перехода)
const chatUrl = await AgentDeskWidget.getChatUrl();
```

Пример создания собственной кнопки вызова поддержки в шапке вашего сайта:
```html
<button type="button" onclick="AgentDeskWidget.open()">
  Написать в поддержку
</button>
```

---

## Content Security Policy (CSP) и CORS

Если на вашем сайте настроена строгая политика безопасности Content Security Policy (CSP), добавьте домен AgentDesk в разрешённые источники:

```http
script-src  https://support.example.com;
frame-src   https://support.example.com;
connect-src https://support.example.com;
```

---

## Диагностика частых проблем

| Проблема | Причина и способ устранения |
| :--- | :--- |
| **Пользователь отображается как «Гость...»** | 1. Проверьте, что передаётся либо валидный `userToken`, либо `externalId` + `signExternalId`.<br>2. Проверьте секретный ключ канала `User Token Secret`.<br>3. Убедитесь, что серверное время на вашем сервере не расходится с сервером AgentDesk (срок действия подписи/токена). |
| **Кнопка виджета не появляется** | В консоли браузера проверьте ошибку: чаще всего не указан `channelId` или скрипт SDK загрузился раньше, чем объявлен `window.AgentDeskConfig`. |
| **Кнопка есть, но внутри окна чата белый экран** | Браузер блокирует загрузку `iframe`. Проверьте заголовки `X-Frame-Options` и `Content-Security-Policy (frame-src)` на вашем прокси-сервере. |
| **Язык виджета не совпадает с сайтом** | Задайте параметр `language: "ru-RU"` (или `en-US`) явно в конфигурации виджета. |
| **Дублирование виджета в SPA-приложениях** | При смене маршрута или размонтировании вызывайте `AgentDeskWidget.destroy()`. |
