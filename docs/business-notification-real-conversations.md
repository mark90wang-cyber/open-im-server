# Business Notification Real Conversations

## Send API

`POST /msg/send_business_notification` sends a structured business notification as an OpenIM message.

Required:

- `recvUserID` or `recvGroupID`: exactly one receiver target.
- `businessType` or `channel`: one of `system`, `prop`, `group_notification`, `social_like`, `social_comment`, `social_follow`, `group_post`.
- `title`
- `summary`

Optional:

- `sendUserID`: defaults by channel when omitted.
- `subType`: for `prop`, use `prop_use`, `prop_acquire`, or `prop_expire`.
- `entityId`, `actorUserId`, `route`, `createdAt`, `dedupeKey`, `payloadData`
- `countUnread`: defaults to `true`; set `false` to save the message and update the conversation preview without increasing unread.
- `offlinePush`: defaults to `true`; set `false` to suppress offline push.
- `senderNickname`, `senderFaceURL`, `offlinePushInfo`

The message content is a `NotificationElem` whose `detail` is:

```json
{
  "payload": {
    "businessType": "prop",
    "subType": "prop_use",
    "entityId": "123",
    "actorUserId": "456",
    "title": "Prop used",
    "summary": "Someone used a prop",
    "route": "PropNotifications",
    "createdAt": 1710000000000,
    "dedupeKey": "prop:123",
    "data": {}
  },
  "delivery": {
    "countUnread": true,
    "offlinePush": true
  }
}
```

## Channel Sender IDs

- `system`: `2`
- `prop`: `6`
- `group_notification`: `7`
- `social_like`: `8`
- `social_comment`: `9`
- `social_follow`: `10`
- `group_post`: `11`

These IDs preserve the existing system/prop behavior while giving new notification channels stable real conversations.

## Prop Subtype Muting

The prop conversation remains a single real OpenIM notification conversation. Business services should read the user's prop notification settings before calling OpenIM:

- Disabled subtype: send with `countUnread=false` and `offlinePush=false`.
- Enabled subtype: send with `countUnread=true` and `offlinePush=true`.

Messages for disabled subtypes are still persisted, so the latest conversation preview can show them. The server advances the receiver's read seq for `countUnread=false` single/notification messages so the saved message does not increase unread.

## Frontend Follow-Up Points

Do these in the Flutter project, not in this repository:

- Parse `latestMsg.notificationElem.detail` or `latestMsg.ex` for `payload.businessType`, falling back to sender IDs for compatibility.
- Route real notification conversations by `businessType`: `system`, `prop`, `group_notification`, `social_like`, `social_comment`, `social_follow`, `group_post`.
- Replace virtual group notification summary once `group_notification` is produced by backend.
- Keep existing `userID == '2'` and `userID == '6'` fallbacks during migration.
- For prop notification settings, call the business settings API for the three subtype switches instead of OpenIM `setConversationMuted`.
- Remove `/api/v1/prop-notifications?includeUnsent=true` fallback after all prop notifications are written through OpenIM.

## Verification Checklist

- New `system`, `prop`, social, group notification, and group post notifications create visible real conversations.
- `countUnread=false` messages are saved and become latest preview but do not increase conversation or total unread.
- `offlinePush=false` messages do not generate offline push.
- `countUnread=true` messages behave like normal unread notification messages.
- Deleting a notification conversation hides it, and a later notification recreates/reappears it.
- Login sync and pull-to-refresh return the same notification conversations without virtual fallback.
