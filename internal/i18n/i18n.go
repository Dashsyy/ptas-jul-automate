// Package i18n centralizes every user-facing string the bot sends — menus,
// prompts, status lines, error messages, usage hints, help text, and the
// tenant invoice template. It's a message catalog, not a translation engine:
// the property only operates in Khmer, so each entry is a constant or a
// small formatting function rather than a keyed lookup table. Keeping all of
// it here (instead of scattered fmt.Sprintf literals through
// internal/service, internal/telegram, and internal/billing) means a wording
// change or a future second language touches one file, and every literal is
// easy to find and review in one place.
package i18n

import "fmt"

// ---- Commands (registered with Telegram via setMyCommands; the command
// names themselves stay in English/ASCII since Telegram requires that, but
// their autocomplete descriptions are Khmer). ----
const (
	CmdStart    = "បើកម៉ឺនុយដើម"
	CmdStatus   = "របាយការណ៍បង់ប្រាក់ប្រចាំខែ"
	CmdUnpaid   = "បន្ទប់ជំពាក់ (ចុចដើម្បីទូទាត់)"
	CmdPay      = "កត់ត្រាការបង់ប្រាក់"
	CmdBilling  = "បន្ទប់មិនទាន់កត់លេខម៉ែត្រ"
	CmdRooms    = "មើលបញ្ជីបន្ទប់ទាំងអស់"
	CmdSetName  = "ដាក់ឈ្មោះអ្នកជួល"
	CmdVacate   = "កត់ត្រាអ្នករើចេញ"
	CmdMoveIn   = "កត់ត្រាអ្នករើចូល"
	CmdNewMonth = "បង្កើតវិក្កយបត្រខែថ្មី"
	CmdTotal    = "សរុបលុយប្រចាំខែ"
	CmdCancel   = "បោះបង់"
	CmdHelp     = "របៀបប្រើប្រាស់រូបូត"
)

// ---- Main menu (the BotFather-style /start screen) ----
const (
	MainMenuHeader = "ជំរាបសួរពី PTAS Bot! 👋 តើថ្ងៃនេះមានអ្វីឲ្យខ្ញុំជួយ?"
	BtnStatus      = "📊 ស្ថានភាព"
	BtnUnpaid      = "💰 នៅជំពាក់"
	BtnBilling     = "📋 គិតលុយ"
	BtnRooms       = "🏠 បន្ទប់"
	BtnPay         = "💵 បង់ប្រាក់"
	BtnSetName     = "✏️ ដាក់ឈ្មោះ"
	BtnVacate      = "🚪 រើចេញ"
	BtnMoveIn      = "🔑 រើចូល"
	BtnNewMonth    = "📅 ខែថ្មី"
	BtnTotal       = "🧮 សរុបលុយ"
	BtnBackMain    = "⬅️ ត្រឡប់ទៅម៉ឺនុយដើម"
	BtnBackRooms   = "⬅️ ត្រឡប់ទៅបន្ទប់ទាំងអស់"
)

// RoomLabel is the tappable-button label used everywhere a bare room number
// is offered as a choice (room picker, setname/vacate/movein pickers, and
// the room submenu's fallback header).
func RoomLabel(roomNumber int) string {
	return fmt.Sprintf("បន្ទប់ទី %d", roomNumber)
}

// ---- Room pickers / submenu prompts ----
const (
	PickRoom            = "សូមជ្រើសរើសបន្ទប់៖"
	PickWhichPaid       = "តើបន្ទប់លេខប៉ុន្មានដែលបានបង់លុយ?"
	PickWhichSetName    = "តើអ្នកចង់ដាក់ឈ្មោះឲ្យបន្ទប់មួយណា?"
	PickWhichVacate     = "តើអ្នកជួលបន្ទប់មួយណាកំពុងរើចេញ?"
	PickWhichMoveIn     = "តើអ្នកជួលថ្មីនឹងស្នាក់នៅបន្ទប់មួយណា?"
	NoOccupiedForVacate = "គ្រប់បន្ទប់ទំនេរអស់ហើយពេលនេះ។"
	NoVacantForMoveIn   = "សុំទោស គ្មានបន្ទប់ទំនេរទេពេលនេះ។"
	NobodyOwes          = "🎉 អបអរសាទរ! គ្មានបន្ទប់ណាជំពាក់លុយទេខែនេះ។"
)

func RoomOwesAmount(roomNumber int, amount float64) string {
	return fmt.Sprintf("បន្ទប់ %d — ជំពាក់ $%.2f", roomNumber, amount)
}

// UnpaidButtonLabel is the /unpaid list's tap-to-settle-in-full button label.
func UnpaidButtonLabel(roomNumber int, amount float64) string {
	return fmt.Sprintf("បន្ទប់ %d — $%.2f", roomNumber, amount)
}

// PayFullButton is the quick-reply button offered alongside the "how much
// did they pay?" prompt, since most tenants pay in full.
func PayFullButton(amount float64) string {
	return fmt.Sprintf("✅ បង់ពេញ ($%.2f)", amount)
}

// ---- Room list / status overview ----
const (
	Occupied           = "✅ មានអ្នកជួល"
	Vacant             = "🚪 ទំនេរ"
	EveryonePaid       = "🎉 គ្រប់គ្នាបានបង់លុយគ្រប់ចំនួនសម្រាប់ខែនេះហើយ។"
	TapToSettle        = "ចុចលើបន្ទប់ដើម្បីទូទាត់ប្រាក់ (ប្រើ /pay សម្រាប់បង់ខ្លះ)៖"
	NoRoomsThisMonth   = "មិនទាន់មានទិន្នន័យបន្ទប់សម្រាប់ខែនេះទេ។"
	AllReadingsEntered = "✅ គ្រប់បន្ទប់បានកត់លេខម៉ែត្ររួចរាល់ហើយ។"
	TapToEnterReadings = "ចុចលើបន្ទប់ដើម្បីកត់លេខម៉ែត្រ៖"
)

// FloorDivider is the "— Floor N —" section header shown before each floor's
// rooms in listings, mirroring the spreadsheet's per-floor sections.
func FloorDivider(floor int) string {
	return fmt.Sprintf("— ជាន់ទី %d —", floor)
}

func RoomLine(roomNumber int, status string, rentUSD float64) string {
	return fmt.Sprintf("បន្ទប់ %d — %s — $%.0f/ខែ", roomNumber, status, rentUSD)
}

func PaidCount(paid, total int) string {
	return fmt.Sprintf("%d ក្នុងចំណោម %d បន្ទប់បានបង់រួចរាល់។", paid, total)
}

func MonthOpened(label string, roomCount int) string {
	return fmt.Sprintf("📅 បានបង្កើតវិក្កយបត្រខែ %s ជាមួយចំនួន %d បន្ទប់។", label, roomCount)
}

// MonthlyTotal summarizes the current month's billing across every room:
// what's owed in total, what's been collected, and what's still outstanding.
func MonthlyTotal(billed, collected float64) string {
	return fmt.Sprintf("🧮 សរុបលុយប្រចាំខែ\n\nត្រូវទូទាត់សរុប៖ $%.2f\nបានទទួលសរុប៖ $%.2f\nនៅខ្វះសរុប៖ $%.2f",
		billed, collected, billed-collected)
}

// ---- Bill status line — shared by /status, the @mention query, and /pay's
// confirmation, so wording only needs to change here. ----

func StatusVacant(roomNumber int) string {
	return fmt.Sprintf("🚪 បន្ទប់ %d — ទំនេរ", roomNumber)
}

func StatusNoCharge(roomNumber int) string {
	return fmt.Sprintf("➖ បន្ទប់ %d — មិនគិតលុយ", roomNumber)
}

func StatusNoReadings(roomNumber int) string {
	return fmt.Sprintf("⏳ បន្ទប់ %d — រង់ចាំកត់លេខទឹកភ្លើង", roomNumber)
}

func StatusPaidWithOverpay(roomNumber int, total, paid, extra float64) string {
	return fmt.Sprintf("✅ បន្ទប់ %d — បានបង់ (ត្រូវបង់ $%.2f, បានទទួល $%.2f, លើស $%.2f)",
		roomNumber, total, paid, extra)
}

func StatusPaid(roomNumber int, total float64) string {
	return fmt.Sprintf("✅ បន្ទប់ %d — បានបង់ ($%.2f)", roomNumber, total)
}

func StatusPartial(roomNumber int, paid, total, remaining float64) string {
	return fmt.Sprintf("🟡 បន្ទប់ %d — បង់ខ្លះ ($%.2f នៃ $%.2f, នៅខ្វះ $%.2f)",
		roomNumber, paid, total, remaining)
}

func StatusUnpaid(roomNumber int, total float64) string {
	return fmt.Sprintf("❌ បន្ទប់ %d — មិនទាន់បង់ ($%.2f)", roomNumber, total)
}

// ---- Generic errors / usage hints ----
const (
	// ErrPrefix is prepended to a raw error inline at most call sites.
	ErrPrefix = "⚠️ មានបញ្ហាបន្តិច៖ "
	// ErrPrefixPlain is the softer unwrapFriendly fallback for an error that
	// isn't one of the known, friendlier cases.
	ErrPrefixPlain = "មានបញ្ហា៖ "

	Cancelled           = "ប្រតិបត្តិការត្រូវបានបោះបង់។"
	UnknownCommand      = "ខ្ញុំមិនស្គាល់ពាក្យបញ្ជានេះទេ។ សូមសាកល្បងវាយ /help។"
	FullySettled        = "✅ ទូទាត់រួចរាល់។"
	UsageSetName        = "របៀបប្រើ៖ /setname <លេខបន្ទប់> <ឈ្មោះ>"
	RoomNumberMustBeInt = "សូមបញ្ចូលលេខបន្ទប់ជាលេខសុទ្ធ។"
	UsageVacate         = "របៀបប្រើ៖ /vacate <លេខបន្ទប់>"
	UsageMoveIn         = "របៀបប្រើ៖ /movein <លេខបន្ទប់>"
	UsagePay            = "របៀបប្រើ៖ /pay <លេខបន្ទប់> <ចំនួនទឹកប្រាក់>"
	AmountMustBeNumber  = "សូមបញ្ចូលចំនួនទឹកប្រាក់ជាលេខ។"
	RoomNotFound        = "រកមិនឃើញបន្ទប់នេះទេ។"
	NoBillingMonth      = "មិនទាន់មានវិក្កយបត្រខែនេះទេ — សូមប្រើពាក្យបញ្ជា /newmonth 2026-09 ជាមុនសិន។"
)

// Error formats err with the soft inline prefix used at most call sites.
func Error(err error) string {
	return ErrPrefix + err.Error()
}

// ErrorPlain formats err with unwrapFriendly's plain fallback prefix.
func ErrorPlain(err error) string {
	return ErrPrefixPlain + err.Error()
}

func SetNameDone(roomNumber int, name string) string {
	return fmt.Sprintf("បានដាក់ឈ្មោះ %q ទៅឲ្យបន្ទប់ %d។", name, roomNumber)
}

func RecordedPayment(amount float64, roomNumber int) string {
	return fmt.Sprintf("💵 បានកត់ត្រាប្រាក់ $%.2f សម្រាប់បន្ទប់ %d។", amount, roomNumber)
}

// MentionUsageHint is shown when the bot is @mentioned in a group without a
// recognizable "room:<n> status" query.
func MentionUsageHint(username string) string {
	return "សាកល្បងប្រើ៖ @" + username + " room:4 status"
}

// UpdateInfoUsageHint is shown when a "update_info" mention doesn't include
// a recognizable room number.
func UpdateInfoUsageHint(username string) string {
	return "សូមប្រើទម្រង់៖ @" + username + " update_info room 4 — ព្រមទាំង reply ទៅសារជូនដំណឹង ABA PayWay ដើម។"
}

// UpdateInfoNeedsReply is shown when "update_info" is used without replying
// to the ABA PayWay notification message it should read the amount from.
const UpdateInfoNeedsReply = "សូម reply (ឆ្លើយតប) ទៅសារជូនដំណឹង ABA PayWay មុននឹងវាយបញ្ជានេះ។"

// UpdateInfoParseFailed is shown when the replied-to message doesn't parse
// as an ABA PayWay notification.
const UpdateInfoParseFailed = "មិនអាចអានព័ត៌មានបង់ប្រាក់ពីសារនោះទេ — សូមប្រាកដថាសារនោះជាសារជូនដំណឹង ABA PayWay ដើម។"

// DuplicateTransaction is shown when the same ABA PayWay transaction (by
// ID) has already been recorded — e.g. "update_info" was replied to the
// same forwarded notification a second time.
const DuplicateTransaction = "⚠️ ប្រតិបត្តិការនេះត្រូវបានកត់ត្រារួចហើយ — សូមពិនិត្យ /status មុននឹងព្យាយាមម្តងទៀត។"

// UpdateInfoRecorded confirms a payment imported from an ABA PayWay
// notification, naming the payer so the owner can visually double-check the
// parse before trusting the recorded amount.
func UpdateInfoRecorded(payer string, amount float64, roomNumber int) string {
	return fmt.Sprintf("💵 បានកត់ត្រា $%.2f ពី %s សម្រាប់បន្ទប់ %d។", amount, payer, roomNumber)
}

// ---- Vacate / move-in / pay / setname / newmonth conversation prompts ----

func VacateNoActiveMonth(roomNumber int) string {
	return fmt.Sprintf("🚪 បន្ទប់ %d ឥឡូវទំនេរហើយ។ ដោយសារមិនទាន់មានការគិតលុយប្រចាំខែ ដូច្នេះមិនមានអ្វីត្រូវទូទាត់ទេ។", roomNumber)
}

func VacatePromptWater(roomNumber, days int, waterPrev string) string {
	return fmt.Sprintf("🚪 បន្ទប់ %d កំពុងរើចេញ — គិតថ្លៃស្នាក់នៅ %d ថ្ងៃសម្រាប់ខែនេះ។\n💧 សូមបញ្ចូលលេខម៉ែត្រទឹក (លេខចាស់៖ %s m³)៖",
		roomNumber, days, waterPrev)
}

func MoveInNoActiveMonth(roomNumber int) string {
	return fmt.Sprintf("🔑 បន្ទប់ %d មានអ្នកជួលហើយ។ តែយើងមិនទាន់បើកខែគិតលុយទេ — សូមប្រើ /newmonth រួចប្រើ /movein %d ម្តងទៀតដើម្បីកត់ត្រាលេខម៉ែត្រថ្មី។",
		roomNumber, roomNumber)
}

func MoveInPromptWater(roomNumber, days int) string {
	return fmt.Sprintf("🔑 បន្ទប់ %d មានអ្នករើចូល — គិតថ្លៃស្នាក់នៅ %d ថ្ងៃសម្រាប់ខែនេះ។\n💧 សូមបញ្ចូលលេខម៉ែត្រទឹកថ្មីបច្ចុប្បន្ន៖",
		roomNumber, days)
}

func MonthAlreadyExists(label string) string {
	return fmt.Sprintf("វិក្កយបត្រខែ %s មានរួចហើយ។", label)
}

const AmountMustBePositive = "ចំនួនទឹកប្រាក់ត្រូវតែធំជាងសូន្យ។"

func RoomNoChargeNoPayment(roomNumber int) string {
	return fmt.Sprintf("បន្ទប់ %d ត្រូវបានកំណត់ថាមិនគិតលុយ — មិនចាំបាច់បង់ប្រាក់ទេ។", roomNumber)
}

// RoomAlreadyPaid guards against recording a second payment on a bill
// that's already settled in full — shown instead of the "how much did they
// pay?" prompt, and returned as an error if a payment is attempted anyway
// (e.g. via a direct /pay <room> <amount> command).
func RoomAlreadyPaid(roomNumber int, total float64) string {
	return fmt.Sprintf("✅ បន្ទប់ %d បានបង់ពេញ ($%.2f) រួចហើយ — មិនចាំបាច់បង់បន្ថែមទេ។", roomNumber, total)
}

const SettledViaUnpaidNote = "បានទូទាត់រួចរាល់តាមរយៈបញ្ជា /unpaid"

func PayAmountPrompt(roomNumber int, remaining float64) string {
	return fmt.Sprintf("💵 បន្ទប់ %d — តើគាត់បង់លុយប៉ុន្មានដែរ? (នៅខ្វះ $%.2f)", roomNumber, remaining)
}

func PayRecorded(amount float64, roomNumber int) string {
	return fmt.Sprintf("💵 បានទទួលប្រាក់ $%.2f ពីបន្ទប់ %d។", amount, roomNumber)
}

func SetNamePrompt(roomNumber int) string {
	return fmt.Sprintf("✏️ បន្ទប់ %d — តើអ្នកជួលឈ្មោះអ្វីដែរ?", roomNumber)
}

const NameRequired = "សូមបញ្ជាក់ឈ្មោះ — សាកល្បងម្តងទៀត។"

const NewMonthLabelPrompt = "📅 តើខែថ្មីនេះមានឈ្មោះអ្វី? (ឧទាហរណ៍៖ 2026-10)"

const MonthLabelRequired = "ត្រូវការបញ្ចូលឈ្មោះខែ — សូមសាកល្បងម្តងទៀត (ឧទាហរណ៍៖ 2026-10)។"

func BillingPromptWater(roomNumber int, waterPrev string) string {
	return fmt.Sprintf("💧 បន្ទប់ %d — សូមបញ្ចូលលេខម៉ែត្រទឹក (លេខចាស់៖ %s m³)៖", roomNumber, waterPrev)
}

const PleaseEnterNumber = "សូមបញ្ចូលជាលេខ។"

func UnknownActionKind(kind string) string {
	return fmt.Sprintf("មានបញ្ហាប្រព័ន្ធ៖ មិនស្គាល់សកម្មភាព %q", kind)
}

func BillingPromptElec(roomNumber int, elecPrev string) string {
	return fmt.Sprintf("⚡ បន្ទប់ %d — សូមបញ្ចូលលេខម៉ែត្រភ្លើង (លេខចាស់៖ %s kWh)៖", roomNumber, elecPrev)
}

const VacateNoteAppended = "\n\n🚪 បន្ទប់នេះឥឡូវទំនេរហើយ។"

const InvoiceSavedPrefix = "✅ បានរក្សាទុកវិក្កយបត្រ។\n\n"

func UnknownStep(step string) string {
	return fmt.Sprintf("មានបញ្ហាប្រព័ន្ធ៖ ជំហានមិនស្គាល់ %q", step)
}

func MoveInPromptElec(roomNumber int) string {
	return fmt.Sprintf("⚡ បន្ទប់ %d — សូមបញ្ចូលលេខម៉ែត្រភ្លើងថ្មីបច្ចុប្បន្ន៖", roomNumber)
}

func MoveInBaselineDone(roomNumber int, water, elec string) string {
	return fmt.Sprintf("✅ បន្ទប់ %d រួចរាល់ — បានកត់ត្រាលេខម៉ែត្រចាប់ផ្តើម (ទឹក %s m³, ភ្លើង %s kWh)។ វានឹងចាប់ផ្តើមគិតលុយធម្មតានៅពេលអ្នកប្រើ /billing លើកក្រោយ។",
		roomNumber, water, elec)
}

// ---- Invoice template (used by internal/billing.RenderKhmerInvoice) ----
const (
	InvoiceTitle       = "វិក្កយបត្របន្ទប់ជួល"
	InvoiceTableHeader = "លេខថ្មី  | លេខចាស់ | ចំនួន | សរុប"
	InvoiceRowWater    = "ទឹក"
	InvoiceRowElec     = "ភ្លើង"
)

func InvoiceMonthLine(period string) string {
	return fmt.Sprintf("ប្រចាំខែ %s", period)
}

func InvoiceRoomLine(roomNumber int) string {
	return fmt.Sprintf("បន្ទប់លេខ %d", roomNumber)
}

func InvoiceRoomRentLine(rentUSD string, daysStayed, daysInMonth int, rentRiel string) string {
	return fmt.Sprintf("ថ្លៃបន្ទប់: $%s × %d/%d ថ្ងៃ = %s ៛ ($%s)", rentUSD, daysStayed, daysInMonth, rentRiel, rentUSD)
}

func InvoiceTotalLine(totalRiel, totalUSD string) string {
	return fmt.Sprintf("សរុបទឹកប្រាក់: %s ៛  /  $%s", totalRiel, totalUSD)
}

// khmerMonths backs MonthYear — January (index 0) through December (index 11).
var khmerMonths = [...]string{
	"មករា", "កុម្ភៈ", "មីនា", "មេសា", "ឧសភា", "មិថុនា",
	"កក្កដា", "សីហា", "កញ្ញា", "តុលា", "វិច្ឆិកា", "ធ្នូ",
}

// MonthYear renders month (1-12) and year in Khmer, e.g. "សីហា 2026".
func MonthYear(month, year int) string {
	if month < 1 || month > 12 {
		return fmt.Sprintf("%d/%d", month, year)
	}
	return fmt.Sprintf("%s %d", khmerMonths[month-1], year)
}

// ---- /help ----
const HelpText = `🤖 ជំនួយការ PTAS Bot សម្រាប់ផ្ទះជួល

/start - បើកម៉ឺនុយចម្បង ដើម្បីគ្រប់គ្រងអ្វីៗទាំងអស់បានយ៉ាងងាយស្រួល។ អ្នកក៏អាចវាយបញ្ជាផ្ទាល់បានដែរ ដូចខាងក្រោម៖

/status    - ពិនិត្យមើលរបាយការណ៍បង់ប្រាក់ប្រចាំខែ
/unpaid    - មើលបន្ទប់ដែលជំពាក់ (ចុចដើម្បីទូទាត់ប្រាក់ងាយៗ)
/pay <បន្ទប់> <លុយ> - កត់ត្រាការបង់ប្រាក់ (បង់ពេញ ឬបង់ខ្លះ)
/billing   - មើល និងបញ្ចូលលេខម៉ែត្រសម្រាប់បន្ទប់ដែលមិនទាន់មាន
/newmonth YYYY-MM - បង្កើតវិក្កយបត្រខែថ្មី
/total     - មើលចំនួនលុយសរុបត្រូវទូទាត់ បានទទួល និងនៅខ្វះ សម្រាប់ខែនេះ
/rooms     - មើលបញ្ជីបន្ទប់ទាំងអស់របស់អ្នក
/setname <បន្ទប់> <ឈ្មោះ> - ដាក់ឈ្មោះអ្នកជួល
/vacate <បន្ទប់> - កត់ត្រាអ្នករើចេញ៖ គណនាថ្ងៃស្នាក់នៅ និងសួរលេខម៉ែត្រចុងក្រោយ
/movein <បន្ទប់> - កត់ត្រាអ្នករើចូល៖ សួរលេខម៉ែត្រថ្មី ដើម្បីចាប់ផ្តើមគិតលុយ
/cancel    - បោះបង់ប្រតិបត្តិការបច្ចុប្បន្ន

💡 នៅក្នុងគ្រុប អ្នកអាច mention ឈ្មោះខ្ញុំ ដើម្បីឆែកស្ថានភាពបន្ទប់បានយ៉ាងរហ័ស។
ឧទាហរណ៍៖ @<bot> room:4 status

💡 ទទួលបានសារជូនដំណឹង ABA PayWay? Forward សារនោះមកបញ្ចូល រួច reply វាដោយវាយ
@<bot> update_info room 4 — ខ្ញុំនឹងអានចំនួនទឹកប្រាក់ និងកត់ត្រាការបង់ប្រាក់ជូនភ្លាម។`
