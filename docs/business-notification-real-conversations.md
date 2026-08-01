# Message-Level Notification Controls

## Responsibility Boundary

OpenIM only provides generic message capabilities. Business meaning, system sender accounts, payload schema, routing, dedupe, expiry, and notification settings are owned by the business system and frontend.

Use:

- `POST /msg/send_msg` for real notification conversations.
- `POST /msg/send_business_notification` for realtime business notifications that do not necessarily create conversation messages.

## Real Notification Conversations

Business services should call `POST /msg/send_msg` with a system account as `sendID` when the goal is to create or update a real user conversation.

Example:

```json
{
  "recvID": "10001",
  "sendID": "6",
  "sessionType": 1,
  "contentType": 110,
  "content": {
    "data": "{\"type\":\"notification_event\",\"title\":\"Notification title\"}",
    "description": "custom notification",
    "extension": "{\"route\":\"NotificationPage\"}"
  },
  "countUnread": false,
  "notOfflinePush": true,
  "offlinePushInfo": {
    "title": "Notification title",
    "desc": "Notification summary",
    "ex": "{}"
  },
  "ex": "{\"type\":\"notification_event\"}"
}
```

### `/msg/send_msg` Controls

- `countUnread`: optional. If omitted or `true`, unread behaves normally. If `false`, the message is persisted and updates the conversation preview, but the receiver's unread count does not increase.
- `notOfflinePush`: existing field. If `true`, the message is persisted but does not trigger offline push.
- `content`: passed through according to `contentType`; use custom message (`contentType=110`) when the business system and frontend own the payload schema.
- `sendID`: can be a system account if that account exists and has permission to send.

## Realtime Business Notifications

`POST /msg/send_business_notification` remains a generic business notification endpoint.

Use it for:

- prop animations
- client realtime callbacks
- effects that should not necessarily create a conversation message
- `sendMsg=false` scenarios

Example:

```json
{
  "sendUserID": "1",
  "recvUserID": "10001",
  "key": "SINGLE_PROP_ACTION",
  "data": "{\"propId\":123}",
  "sendMsg": false
}
```

Optional controls:

- `sendMsg`: whether this notification should also produce a stored message.
- `countUnread`: applies only when `sendMsg=true`.
- `offlinePush`: applies only when push is desired for this notification path.

This endpoint does not define business channels, system sender IDs, routes, or frontend display behavior.

## Business System Responsibilities

The business system must decide:

- which system account sends each real conversation message
- payload format for `content` and `ex`
- whether a message counts unread
- whether a message triggers offline push
- dedupe and expiry behavior
- group approval state and repeated request handling

## Frontend Responsibilities

The frontend must decide:

- which sender IDs receive special display
- how to parse custom message payloads
- which page to open on tap
- how to render title, avatar, and preview
- how to fetch latest approval state for group approval flows

## Verification Checklist

- `/msg/send_msg` with a system `sendID` creates or updates a real single conversation.
- `countUnread=false` messages are saved and become latest preview but do not increase conversation or total unread.
- `notOfflinePush=true` messages do not generate offline push.
- `countUnread=true` or omitted behaves like a normal unread message.
- Deleting a notification conversation hides it, and a later notification recreates/reappears it.
- `/msg/send_business_notification` with `sendMsg=false` continues to work for realtime callback scenarios.
