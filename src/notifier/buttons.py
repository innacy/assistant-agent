from telegram import InlineKeyboardButton, InlineKeyboardMarkup

ACTION_DONE = "done"
ACTION_SNOOZE_1D = "snooze_1d"
ACTION_SNOOZE_3D = "snooze_3d"
ACTION_SNOOZE_1W = "snooze_1w"
ACTION_DISMISS = "dismiss"
ACTION_DETAILS = "details"


def item_buttons(item_id: str) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton("Mark Done", callback_data=f"{ACTION_DONE}:{item_id}"),
                InlineKeyboardButton("Snooze 1d", callback_data=f"{ACTION_SNOOZE_1D}:{item_id}"),
            ],
            [
                InlineKeyboardButton("Snooze 3d", callback_data=f"{ACTION_SNOOZE_3D}:{item_id}"),
                InlineKeyboardButton("Snooze 1w", callback_data=f"{ACTION_SNOOZE_1W}:{item_id}"),
            ],
            [
                InlineKeyboardButton("Dismiss", callback_data=f"{ACTION_DISMISS}:{item_id}"),
                InlineKeyboardButton("Details", callback_data=f"{ACTION_DETAILS}:{item_id}"),
            ],
        ]
    )


def snooze_buttons(item_id: str) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton("1 day", callback_data=f"{ACTION_SNOOZE_1D}:{item_id}"),
                InlineKeyboardButton("3 days", callback_data=f"{ACTION_SNOOZE_3D}:{item_id}"),
                InlineKeyboardButton("1 week", callback_data=f"{ACTION_SNOOZE_1W}:{item_id}"),
            ],
        ]
    )


def parse_callback(data: str) -> tuple[str, str]:
    parts = data.split(":", 1)
    if len(parts) == 2:
        return parts[0], parts[1]
    return data, ""
