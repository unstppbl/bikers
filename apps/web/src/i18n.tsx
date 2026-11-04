import { createContext, useContext, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { ApiError } from './api'
import type { Locale } from './types'
import { phase3En, phase3Kk, phase3Ru } from './phase3-i18n'
import { photosEn, photosKk, photosRu } from './photos-i18n'
import { installEn, installKk, installRu } from './install-i18n'
import { preferencesEn, preferencesKk, preferencesRu } from './preferences-i18n'
import { mvpEn, mvpKk, mvpRu } from './mvp-i18n'

const en = {
  ...mvpEn,
  ...preferencesEn,
  ...installEn,
  ...photosEn,
  ...phase3En,
  rides: 'Rides', profile: 'My profile', signIn: 'Sign in', signOut: 'Sign out', newRide: 'Create a ride',
  locale: 'Language', menu: 'Navigation', close: 'Close', back: 'Back to rides', retry: 'Try again',
  community: 'The motorcycle community', country: 'Kazakhstan', together: 'A good ride starts with good company.',
  sidebarNote: 'Find your people. Make a plan. Ride together.', chapter: '01 / ON THE ROAD',
  heroFirst: 'Find your', heroSecond: 'next ride.', heroText: 'Discover local rides, meet fellow riders, and keep the whole plan in one place.',
  browse: 'Find a ride', upcoming: 'Rides to join', searchLabel: 'Filter rides', searchPlaceholder: 'Search by city or description',
  availableRides: '{count} rides', organizerApproval: 'Organizer approval', viewRide: 'View ride',
  emptyTitle: 'The road is open.', emptyText: 'No rides have been published yet. Bring your people together and create the first one.',
  noResults: 'No rides match your search.', noResultsText: 'Try another city or clear the filter.', clearFilter: 'Clear filter',
  howItWorks: 'From a plan to a shared ride', stepOne: 'Find your ride', stepOneText: 'Choose a city and a plan that feels right.',
  stepTwo: 'Request a place', stepTwoText: 'The organizer approves each rider before sharing meeting details.',
  stepThree: 'Meet your people', stepThreeText: 'Keep the current plan and conversation together in the Ride Room.',
  privacyLabel: 'Your location stays yours', privacyText: 'Browsing never requires location access. Exact meeting details stay inside approved rides.',
  loading: 'Loading…', loadingSession: 'Checking your session…', serviceUnavailable: 'We couldn’t reach BIKERS.KZ.',
  serviceText: 'Check your connection and try again.', offline: 'You’re offline. Reconnect to load rides and make changes.',
  updateReady: 'A new version is ready.', updateNow: 'Update app', later: 'Later',
  loginEyebrow: 'YOUR RIDING COMMUNITY', loginTitle: 'Start with your people.', loginText: 'Sign in or create an account with a code delivered through Telegram.',
  phone: 'Phone number', phoneHint: 'Use a phone number registered with Telegram. Country code required.', phonePlaceholder: '+7 700 000 00 00',
  sendCode: 'Send Telegram code', sending: 'Sending…', codeTitle: 'Check Telegram.', codeText: 'Enter the code sent to {phone}.',
  code: 'Verification code', verify: 'Verify and continue', verifying: 'Verifying…', changeNumber: 'Use another number',
  resend: 'Send a new code', resendIn: 'Resend in {seconds}s', expiredCode: 'This code has expired. Request a new one.',
  telegramOnly: 'Telegram is the only sign-in method. Your phone number is private.',
  profileEyebrow: 'A LITTLE ABOUT YOU', profileTitle: 'Make yourself at home.', profileEditTitle: 'Your rider profile.',
  profileText: 'A name and city help other riders recognize you. Owning a motorcycle is optional.', displayName: 'Display name',
  namePlaceholder: 'What should riders call you?', city: 'City', cityPlaceholder: 'Your city', languages: 'Languages you speak',
  interests: 'Riding interests', interestsPlaceholder: 'City rides, touring, a first motorcycle…', optional: 'Optional',
  saveProfile: 'Save profile', continueRides: 'Save and find a ride', saving: 'Saving…', saved: 'Changes saved.',
  publicProfileNote: 'Your display name is shared with approved ride participants. Your phone number is never part of your public profile.',
  ridePreview: 'PUBLIC RIDE PREVIEW', rideIn: 'A ride in {city}', exactDetails: 'The details stay with the group.',
  previewText: 'Request to join. Once the organizer approves you, the plan, participants, and Ride Room will open here.',
  requestJoin: 'Request to join', signInJoin: 'Sign in to request a place', pendingTitle: 'Your request is with the organizer.',
  pendingText: 'This page will update when your place is approved. Meeting details stay private until then.',
  requestAgain: 'Request again', rejectedTitle: 'Your request wasn’t approved.', removedTitle: 'You no longer have access to this ride.',
  leaveRide: 'Leave ride', leaveConfirm: 'Leave this ride? You will lose access to its meeting details and Ride Room.',
  requestSent: 'Join request sent.', cancelled: 'Cancelled', published: 'Open for requests', requested: 'Requested', approved: 'Approved',
  rejected: 'Not approved', removed: 'Removed', left: 'Left', draft: 'Draft', cancelRide: 'Cancel ride',
  cancelConfirm: 'Cancel this ride? Participants will see that the plan is cancelled.', cancelledText: 'This ride has been cancelled. The plan and conversation remain available to authorized participants.',
  ridePlan: 'The current plan', meetingPoint: 'Meeting point', destination: 'Destination', expectations: 'Riding expectations',
  routeLink: 'Open organizer’s route', navigate: 'Open in maps', departure: 'Departure', capacity: 'Participant limit', capacityHint: 'Includes the organizer',
  participants: 'Participants', joinRequests: 'Join requests', noRequests: 'No requests waiting for approval.', noParticipants: 'No participants to show.',
  approve: 'Approve', reject: 'Decline', remove: 'Remove', removeConfirm: 'Remove this rider? Their access to the room and meeting details will end.',
  you: 'You', organizer: 'Organizer', editRide: 'Edit plan', rideRoom: 'Ride Room', roomText: 'One place for everyone riding together.',
  messagesEmpty: 'Start the conversation.', messagesEmptyText: 'Coordinate with the group here. The current ride plan stays above the conversation.',
  messageLabel: 'Message the Ride Room', messagePlaceholder: 'Write to the group…', send: 'Send message', sendingMessage: 'Sending…',
  connected: 'Live updates connected', connecting: 'Connecting live updates…', connectionLost: 'Rechecking access and connection…',
  roomReadOnly: 'This ride is cancelled. The conversation is read-only.', loadMessagesError: 'Couldn’t load the conversation.',
  createEyebrow: 'BRING YOUR PEOPLE TOGETHER', createTitle: 'Make a plan worth joining.', editTitle: 'Keep everyone on the same page.',
  createText: 'Share a short public preview. Meeting details and the Ride Room are only visible after you approve a rider.',
  title: 'Ride title', titlePlaceholder: 'Give the group a plan to look forward to', description: 'Public description',
  descriptionPlaceholder: 'A short introduction for riders browsing in your city', publicFields: 'The public preview', privateFields: 'For approved participants',
  privateFieldsHint: 'These details stay private until the organizer approves a join request.', publicFieldsHint: 'Only the city, date, and short description appear in public previews.',
  startsAt: 'Date and time', timezoneHint: 'Almaty time · UTC+5', meetingPlaceholder: 'A clear place to meet',
  destinationPlaceholder: 'Where are you heading?', route: 'Organizer’s route link', routePlaceholder: 'https://…',
  expectationsPlaceholder: 'Pace, experience, road surface, and what to bring', create: 'Publish ride', savePlan: 'Save plan',
  approvalNote: 'You approve every join request. Pending requests do not reserve a place.', noAccess: 'This page isn’t available to your account.',
  notFound: 'That page couldn’t be found.', requiredFields: 'Please fill in all required fields.', invalidPhone: 'Enter a phone number with its country code.',
  invalidCode: 'Enter the code from Telegram.', invalidTime: 'Choose a valid future date and time.', invalidCapacity: 'Choose a participant limit from 1 to 1000.',
  invalidUrl: 'Use a complete HTTP or HTTPS link.', errorGeneric: 'Something went wrong. Please try again.',
  errorNetwork: 'Connection interrupted. Check your connection and try again.', errorUnauthorized: 'Your session ended. Please sign in again.',
  errorForbidden: 'You no longer have permission to do that.', errorRateLimit: 'Too many attempts. Please wait before trying again.',
  errorOtpInvalid: 'That code wasn’t accepted. Check the code in Telegram and try again.', errorOtpExpired: 'This code has expired. Request a new one.',
  errorOtpUnavailable: 'Telegram delivery is unavailable right now. Please try again later.', errorConflict: 'The plan changed. Reload the latest version before saving.',
  errorRideFull: 'This ride has reached its participant limit.', errorCancelled: 'This ride is cancelled.', errorValidation: 'Check your information and try again.',
  characterCount: '{count} / {max}', dismiss: 'Dismiss', notNow: 'Not now', refreshing: 'Refreshing…', requiredLanguage: 'Select at least one language you speak.', errorSessionRefresh: 'Refresh this page and sign in again before continuing.',
  errorProfileRequired: 'Complete your rider profile before continuing.', errorCapacityTooSmall: 'The limit cannot be lower than the number of approved participants.',
  errorParticipationChanged: 'This join request changed. Refresh the participants and try again.', errorMessageConflict: 'Couldn’t confirm this message. Reload the conversation before sending again.',
  errorOrganizerCannotLeave: 'The organizer must cancel the ride instead of leaving it.', errorAccountUnavailable: 'This account is not available. Please contact the operator.',
} as const

type Key = keyof typeof en
const ru: Record<Key, string> = {
  ...mvpRu,
  ...preferencesRu,
  ...installRu,
  ...photosRu,
  ...phase3Ru,
  rides: 'Поездки', profile: 'Мой профиль', signIn: 'Войти', signOut: 'Выйти', newRide: 'Создать поездку',
  locale: 'Язык', menu: 'Навигация', close: 'Закрыть', back: 'К поездкам', retry: 'Попробовать снова',
  community: 'Мотосообщество', country: 'Казахстан', together: 'Хорошая поездка начинается с хорошей компании.',
  sidebarNote: 'Найдите своих. Составьте план. Поезжайте вместе.', chapter: '01 / В ДОРОГУ',
  heroFirst: 'Найдите', heroSecond: 'свою поездку.', heroText: 'Находите поездки рядом, знакомьтесь с мотоциклистами и обсуждайте план в одном месте.',
  browse: 'Найти поездку', upcoming: 'Поездки для вас', searchLabel: 'Фильтр поездок', searchPlaceholder: 'Поиск по городу или описанию',
  availableRides: 'Поездок: {count}', organizerApproval: 'Одобрение организатора', viewRide: 'Открыть поездку',
  emptyTitle: 'Дорога открыта.', emptyText: 'Пока нет опубликованных поездок. Соберите своих и создайте первую.',
  noResults: 'Поездки не найдены.', noResultsText: 'Попробуйте другой город или сбросьте фильтр.', clearFilter: 'Сбросить фильтр',
  howItWorks: 'От плана к общей поездке', stepOne: 'Найдите поездку', stepOneText: 'Выберите город и подходящий план.',
  stepTwo: 'Подайте заявку', stepTwoText: 'Организатор одобрит заявку, прежде чем откроются детали встречи.',
  stepThree: 'Встречайте своих', stepThreeText: 'План и общение группы собраны в комнате поездки.',
  privacyLabel: 'Местоположение под вашим контролем', privacyText: 'Для просмотра не нужен доступ к геолокации. Точное место встречи доступно участникам после одобрения.',
  loading: 'Загрузка…', loadingSession: 'Проверяем сессию…', serviceUnavailable: 'Не удалось связаться с BIKERS.KZ.',
  serviceText: 'Проверьте подключение и попробуйте снова.', offline: 'Вы не в сети. Для загрузки и изменений нужно подключение.',
  updateReady: 'Доступна новая версия.', updateNow: 'Обновить', later: 'Позже',
  loginEyebrow: 'ВАШЕ МОТОСООБЩЕСТВО', loginTitle: 'Всё начинается со своих.', loginText: 'Войдите или создайте аккаунт с помощью кода в Telegram.',
  phone: 'Номер телефона', phoneHint: 'Укажите номер, зарегистрированный в Telegram, с кодом страны.', phonePlaceholder: '+7 700 000 00 00',
  sendCode: 'Получить код в Telegram', sending: 'Отправляем…', codeTitle: 'Проверьте Telegram.', codeText: 'Введите код, отправленный на {phone}.',
  code: 'Код подтверждения', verify: 'Подтвердить и продолжить', verifying: 'Проверяем…', changeNumber: 'Другой номер',
  resend: 'Отправить новый код', resendIn: 'Повторить через {seconds} с', expiredCode: 'Срок действия кода истёк. Запросите новый.',
  telegramOnly: 'Вход доступен только через Telegram. Ваш номер телефона скрыт.',
  profileEyebrow: 'НЕМНОГО О ВАС', profileTitle: 'Рады знакомству.', profileEditTitle: 'Ваш профиль.',
  profileText: 'Имя и город помогут другим участникам узнать вас. Мотоцикл не обязателен.', displayName: 'Отображаемое имя',
  namePlaceholder: 'Как к вам обращаться?', city: 'Город', cityPlaceholder: 'Ваш город', languages: 'Языки общения',
  interests: 'Мотоинтересы', interestsPlaceholder: 'Городские поездки, туризм, первый мотоцикл…', optional: 'Необязательно',
  saveProfile: 'Сохранить профиль', continueRides: 'Сохранить и найти поездку', saving: 'Сохраняем…', saved: 'Изменения сохранены.',
  publicProfileNote: 'Участники одобренных поездок видят ваше имя. Номер телефона не отображается в публичном профиле.',
  ridePreview: 'ПУБЛИЧНЫЙ АНОНС', rideIn: 'Поездка в городе {city}', exactDetails: 'Подробности — для группы.',
  previewText: 'Подайте заявку. После одобрения организатором здесь откроются план, участники и комната поездки.',
  requestJoin: 'Подать заявку', signInJoin: 'Войти и подать заявку', pendingTitle: 'Заявка у организатора.',
  pendingText: 'Страница обновится после одобрения. До этого детали встречи скрыты.',
  requestAgain: 'Подать повторно', rejectedTitle: 'Заявка не одобрена.', removedTitle: 'У вас больше нет доступа к поездке.',
  leaveRide: 'Покинуть поездку', leaveConfirm: 'Покинуть поездку? Доступ к деталям встречи и комнате будет закрыт.',
  requestSent: 'Заявка отправлена.', cancelled: 'Отменена', published: 'Заявки открыты', requested: 'Заявка подана', approved: 'Одобрен',
  rejected: 'Не одобрен', removed: 'Удалён', left: 'Вышел', draft: 'Черновик', cancelRide: 'Отменить поездку',
  cancelConfirm: 'Отменить поездку? Участники увидят, что план отменён.', cancelledText: 'Поездка отменена. План и история общения доступны участникам с разрешённым доступом.',
  ridePlan: 'Актуальный план', meetingPoint: 'Место встречи', destination: 'Куда едем', expectations: 'Условия поездки',
  routeLink: 'Маршрут организатора', navigate: 'Открыть карту', departure: 'Отправление', capacity: 'Лимит участников', capacityHint: 'Включая организатора',
  participants: 'Участники', joinRequests: 'Заявки на участие', noRequests: 'Нет заявок на рассмотрении.', noParticipants: 'Пока нет участников.',
  approve: 'Одобрить', reject: 'Отклонить', remove: 'Удалить', removeConfirm: 'Удалить участника? Доступ к комнате и деталям встречи будет закрыт.',
  you: 'Вы', organizer: 'Организатор', editRide: 'Изменить план', rideRoom: 'Комната поездки', roomText: 'Одно место для всех, кто едет вместе.',
  messagesEmpty: 'Начните разговор.', messagesEmptyText: 'Обсудите поездку с группой. Актуальный план находится над перепиской.',
  messageLabel: 'Сообщение в комнату поездки', messagePlaceholder: 'Напишите группе…', send: 'Отправить сообщение', sendingMessage: 'Отправляем…',
  connected: 'Обновления подключены', connecting: 'Подключаем обновления…', connectionLost: 'Проверяем доступ и подключение…',
  roomReadOnly: 'Поездка отменена. Переписка доступна только для чтения.', loadMessagesError: 'Не удалось загрузить переписку.',
  createEyebrow: 'СОБЕРИТЕ СВОИХ', createTitle: 'Создайте план, к которому хочется присоединиться.', editTitle: 'Держите группу в курсе.',
  createText: 'Опубликуйте короткий анонс. Детали встречи и комната доступны только после одобрения участника.',
  title: 'Название поездки', titlePlaceholder: 'Какой план ждёт группу?', description: 'Публичное описание',
  descriptionPlaceholder: 'Короткое приглашение для мотоциклистов в вашем городе', publicFields: 'Публичный анонс', privateFields: 'Для одобренных участников',
  privateFieldsHint: 'Эти детали скрыты до одобрения заявки организатором.', publicFieldsHint: 'В публичном анонсе видны только город, дата и краткое описание.',
  startsAt: 'Дата и время', timezoneHint: 'Время Алматы · UTC+5', meetingPlaceholder: 'Понятное место сбора',
  destinationPlaceholder: 'Куда направимся?', route: 'Ссылка на маршрут организатора', routePlaceholder: 'https://…',
  expectationsPlaceholder: 'Темп, опыт, покрытие дороги и что взять с собой', create: 'Опубликовать поездку', savePlan: 'Сохранить план',
  approvalNote: 'Каждую заявку одобряете вы. Заявки не резервируют места.', noAccess: 'Эта страница недоступна вашему аккаунту.',
  notFound: 'Страница не найдена.', requiredFields: 'Заполните все обязательные поля.', invalidPhone: 'Введите номер с кодом страны.',
  invalidCode: 'Введите код из Telegram.', invalidTime: 'Выберите корректные дату и время в будущем.', invalidCapacity: 'Укажите лимит от 1 до 1000 участников.',
  invalidUrl: 'Укажите полную ссылку HTTP или HTTPS.', errorGeneric: 'Что-то пошло не так. Попробуйте снова.',
  errorNetwork: 'Соединение прервано. Проверьте подключение и попробуйте снова.', errorUnauthorized: 'Сессия завершена. Войдите снова.',
  errorForbidden: 'У вас больше нет разрешения на это действие.', errorRateLimit: 'Слишком много попыток. Подождите и повторите.',
  errorOtpInvalid: 'Код не принят. Проверьте код в Telegram и повторите.', errorOtpExpired: 'Срок действия кода истёк. Запросите новый.',
  errorOtpUnavailable: 'Доставка через Telegram сейчас недоступна. Попробуйте позже.', errorConflict: 'План изменился. Обновите его перед сохранением.',
  errorRideFull: 'Достигнут лимит участников.', errorCancelled: 'Поездка отменена.', errorValidation: 'Проверьте данные и попробуйте снова.',
  characterCount: '{count} / {max}', dismiss: 'Закрыть', notNow: 'Не сейчас', refreshing: 'Обновляем…', requiredLanguage: 'Выберите хотя бы один язык общения.', errorSessionRefresh: 'Обновите страницу и войдите снова, чтобы продолжить.',
  errorProfileRequired: 'Заполните профиль, чтобы продолжить.', errorCapacityTooSmall: 'Лимит не может быть меньше числа одобренных участников.',
  errorParticipationChanged: 'Заявка изменилась. Обновите список участников и повторите.', errorMessageConflict: 'Не удалось подтвердить сообщение. Обновите переписку перед повторной отправкой.',
  errorOrganizerCannotLeave: 'Организатор должен отменить поездку, а не покидать её.', errorAccountUnavailable: 'Аккаунт недоступен. Обратитесь к оператору.',
}

const kk: Record<Key, string> = {
  ...mvpKk,
  ...preferencesKk,
  ...installKk,
  ...photosKk,
  ...phase3Kk,
  rides: 'Сапарлар', profile: 'Менің профилім', signIn: 'Кіру', signOut: 'Шығу', newRide: 'Сапар құру',
  locale: 'Тіл', menu: 'Навигация', close: 'Жабу', back: 'Сапарларға оралу', retry: 'Қайталау',
  community: 'Мотоқауымдастық', country: 'Қазақстан', together: 'Жақсы сапар жақсы серіктен басталады.',
  sidebarNote: 'Өз адамдарыңызды табыңыз. Жоспар құрыңыз. Бірге жүріңіз.', chapter: '01 / ЖОЛҒА ШЫҒАЙЫҚ',
  heroFirst: 'Келесі', heroSecond: 'сапарыңыз.', heroText: 'Жақын жердегі сапарларды табыңыз, мотоциклшілермен танысыңыз және жоспарды бірге талқылаңыз.',
  browse: 'Сапар табу', upcoming: 'Қосылуға болатын сапарлар', searchLabel: 'Сапарларды сүзу', searchPlaceholder: 'Қала немесе сипаттама бойынша іздеу',
  availableRides: 'Сапар саны: {count}', organizerApproval: 'Ұйымдастырушының мақұлдауы', viewRide: 'Сапарды ашу',
  emptyTitle: 'Жол ашық.', emptyText: 'Әзірге жарияланған сапар жоқ. Өз адамдарыңызды жинап, алғашқы сапарды құрыңыз.',
  noResults: 'Сапар табылмады.', noResultsText: 'Басқа қаланы көріңіз немесе сүзгіні тазалаңыз.', clearFilter: 'Сүзгіні тазалау',
  howItWorks: 'Жоспардан ортақ сапарға', stepOne: 'Сапарды табыңыз', stepOneText: 'Қала мен өзіңізге лайық жоспарды таңдаңыз.',
  stepTwo: 'Өтінім жіберіңіз', stepTwoText: 'Кездесу мәліметтері ұйымдастырушы мақұлдағаннан кейін ашылады.',
  stepThree: 'Өз адамдарыңызбен кездесіңіз', stepThreeText: 'Жоспар мен топ әңгімесі сапар бөлмесінде жиналады.',
  privacyLabel: 'Орналасқан жеріңіз өз бақылауыңызда', privacyText: 'Сапарларды көру үшін геолокация қажет емес. Нақты кездесу орны мақұлданған қатысушыларға ғана ашылады.',
  loading: 'Жүктелуде…', loadingSession: 'Сессия тексерілуде…', serviceUnavailable: 'BIKERS.KZ қызметіне қосылу мүмкін болмады.',
  serviceText: 'Байланысты тексеріп, қайталап көріңіз.', offline: 'Желіге қосылмағансыз. Жүктеу және өзгерту үшін қайта қосылыңыз.',
  updateReady: 'Жаңа нұсқа дайын.', updateNow: 'Жаңарту', later: 'Кейінірек',
  loginEyebrow: 'СІЗДІҢ МОТОҚАУЫМДАСТЫҒЫҢЫЗ', loginTitle: 'Бәрі өз адамдарыңыздан басталады.', loginText: 'Telegram арқылы келген кодпен кіріңіз немесе аккаунт ашыңыз.',
  phone: 'Телефон нөмірі', phoneHint: 'Telegram-да тіркелген нөмірді ел кодымен бірге енгізіңіз.', phonePlaceholder: '+7 700 000 00 00',
  sendCode: 'Telegram кодын алу', sending: 'Жіберілуде…', codeTitle: 'Telegram-ды тексеріңіз.', codeText: '{phone} нөміріне келген кодты енгізіңіз.',
  code: 'Растау коды', verify: 'Растау және жалғастыру', verifying: 'Тексерілуде…', changeNumber: 'Басқа нөмірді қолдану',
  resend: 'Жаңа код жіберу', resendIn: '{seconds} секундтан кейін қайталау', expiredCode: 'Кодтың мерзімі аяқталды. Жаңасын сұраңыз.',
  telegramOnly: 'Кіру тек Telegram арқылы қолжетімді. Телефон нөміріңіз жасырын.',
  profileEyebrow: 'ӨЗІҢІЗ ТУРАЛЫ', profileTitle: 'Танысқаныма қуаныштымыз.', profileEditTitle: 'Сіздің профиліңіз.',
  profileText: 'Атыңыз бен қалаңыз басқа қатысушыларға сізді тануға көмектеседі. Мотоциклдің болуы міндетті емес.', displayName: 'Көрсетілетін ат',
  namePlaceholder: 'Сізді қалай атаймыз?', city: 'Қала', cityPlaceholder: 'Сіздің қалаңыз', languages: 'Сөйлейтін тілдеріңіз',
  interests: 'Мотоқызығушылықтар', interestsPlaceholder: 'Қала сапарлары, саяхат, алғашқы мотоцикл…', optional: 'Міндетті емес',
  saveProfile: 'Профильді сақтау', continueRides: 'Сақтау және сапар табу', saving: 'Сақталуда…', saved: 'Өзгерістер сақталды.',
  publicProfileNote: 'Мақұлданған сапар қатысушылары атыңызды көреді. Телефон нөміріңіз ашық профильде көрсетілмейді.',
  ridePreview: 'АШЫҚ САПАР АНОНСЫ', rideIn: '{city} қаласындағы сапар', exactDetails: 'Толық мәліметтер топқа арналған.',
  previewText: 'Қосылуға өтінім жіберіңіз. Ұйымдастырушы мақұлдағаннан кейін жоспар, қатысушылар және сапар бөлмесі ашылады.',
  requestJoin: 'Қосылуға өтінім беру', signInJoin: 'Кіру және өтінім беру', pendingTitle: 'Өтініміңіз ұйымдастырушыда.',
  pendingText: 'Өтінім мақұлданғанда бет жаңартылады. Оған дейін кездесу мәліметтері жасырын.',
  requestAgain: 'Қайта өтінім беру', rejectedTitle: 'Өтінім мақұлданбады.', removedTitle: 'Бұл сапарға енді қолжетімділігіңіз жоқ.',
  leaveRide: 'Сапардан шығу', leaveConfirm: 'Сапардан шығасыз ба? Кездесу мәліметтері мен сапар бөлмесіне қолжетімділік жабылады.',
  requestSent: 'Өтінім жіберілді.', cancelled: 'Бас тартылды', published: 'Өтінімдер қабылданады', requested: 'Өтінім жіберілді', approved: 'Мақұлданды',
  rejected: 'Мақұлданбады', removed: 'Шеттетілді', left: 'Шықты', draft: 'Жоба', cancelRide: 'Сапардан бас тарту',
  cancelConfirm: 'Сапардан бас тартасыз ба? Қатысушылар жоспардың тоқтатылғанын көреді.', cancelledText: 'Сапардан бас тартылды. Жоспар мен әңгіме тарихы рұқсаты бар қатысушыларға қолжетімді.',
  ridePlan: 'Өзекті жоспар', meetingPoint: 'Кездесу орны', destination: 'Баратын жер', expectations: 'Сапар шарттары',
  routeLink: 'Ұйымдастырушының бағытын ашу', navigate: 'Картадан ашу', departure: 'Жолға шығу', capacity: 'Қатысушылар лимиті', capacityHint: 'Ұйымдастырушыны қоса алғанда',
  participants: 'Қатысушылар', joinRequests: 'Қосылу өтінімдері', noRequests: 'Қаралатын өтінім жоқ.', noParticipants: 'Әзірге қатысушылар жоқ.',
  approve: 'Мақұлдау', reject: 'Қабылдамау', remove: 'Шеттету', removeConfirm: 'Қатысушыны шеттетесіз бе? Бөлме мен кездесу мәліметтеріне қолжетімділігі жабылады.',
  you: 'Сіз', organizer: 'Ұйымдастырушы', editRide: 'Жоспарды өзгерту', rideRoom: 'Сапар бөлмесі', roomText: 'Бірге жүретіндер үшін бір орын.',
  messagesEmpty: 'Әңгімені бастаңыз.', messagesEmptyText: 'Сапарды топпен осында талқылаңыз. Өзекті жоспар хат алмасудың үстінде орналасқан.',
  messageLabel: 'Сапар бөлмесіне хабарлама', messagePlaceholder: 'Топқа жазыңыз…', send: 'Хабарлама жіберу', sendingMessage: 'Жіберілуде…',
  connected: 'Жаңартулар қосылды', connecting: 'Жаңартулар қосылуда…', connectionLost: 'Қолжетімділік пен байланыс тексерілуде…',
  roomReadOnly: 'Сапардан бас тартылды. Әңгіме тек оқуға қолжетімді.', loadMessagesError: 'Әңгімені жүктеу мүмкін болмады.',
  createEyebrow: 'ӨЗ АДАМДАРЫҢЫЗДЫ ЖИНАҢЫЗ', createTitle: 'Қосылғың келетін жоспар құрыңыз.', editTitle: 'Топты хабардар етіп отырыңыз.',
  createText: 'Қысқа анонс жариялаңыз. Кездесу мәліметтері мен бөлме қатысушы мақұлданғаннан кейін ашылады.',
  title: 'Сапар атауы', titlePlaceholder: 'Топ қандай жоспарды күтеді?', description: 'Ашық сипаттама',
  descriptionPlaceholder: 'Қалаңыздағы мотоциклшілерге қысқа шақыру', publicFields: 'Ашық анонс', privateFields: 'Мақұлданған қатысушыларға',
  privateFieldsHint: 'Бұл мәліметтер ұйымдастырушы өтінімді мақұлдағанға дейін жасырын.', publicFieldsHint: 'Ашық анонста тек қала, күн және қысқа сипаттама көрінеді.',
  startsAt: 'Күні мен уақыты', timezoneHint: 'Алматы уақыты · UTC+5', meetingPlaceholder: 'Түсінікті кездесу орны',
  destinationPlaceholder: 'Қайда барамыз?', route: 'Ұйымдастырушының бағыт сілтемесі', routePlaceholder: 'https://…',
  expectationsPlaceholder: 'Қарқын, тәжірибе, жол жабыны және қажетті заттар', create: 'Сапарды жариялау', savePlan: 'Жоспарды сақтау',
  approvalNote: 'Әр өтінімді өзіңіз мақұлдайсыз. Күтудегі өтінімдер орын сақтамайды.', noAccess: 'Бұл бет аккаунтыңызға қолжетімсіз.',
  notFound: 'Бет табылмады.', requiredFields: 'Барлық міндетті өрістерді толтырыңыз.', invalidPhone: 'Нөмірді ел кодымен бірге енгізіңіз.',
  invalidCode: 'Telegram-дағы кодты енгізіңіз.', invalidTime: 'Болашақтағы дұрыс күн мен уақытты таңдаңыз.', invalidCapacity: 'Қатысушылар лимиті 1 мен 1000 аралығында болуы керек.',
  invalidUrl: 'Толық HTTP немесе HTTPS сілтемесін енгізіңіз.', errorGeneric: 'Қате орын алды. Қайталап көріңіз.',
  errorNetwork: 'Байланыс үзілді. Қосылымды тексеріп, қайталап көріңіз.', errorUnauthorized: 'Сессия аяқталды. Қайта кіріңіз.',
  errorForbidden: 'Бұл әрекетке енді рұқсатыңыз жоқ.', errorRateLimit: 'Тым көп әрекет жасалды. Күтіп, қайталаңыз.',
  errorOtpInvalid: 'Код қабылданбады. Telegram-дағы кодты тексеріп, қайталаңыз.', errorOtpExpired: 'Кодтың мерзімі аяқталды. Жаңасын сұраңыз.',
  errorOtpUnavailable: 'Telegram арқылы жеткізу қазір қолжетімсіз. Кейінірек қайталаңыз.', errorConflict: 'Жоспар өзгерді. Сақтау алдында жаңа нұсқасын жүктеңіз.',
  errorRideFull: 'Қатысушылар лимитіне жетті.', errorCancelled: 'Сапардан бас тартылды.', errorValidation: 'Мәліметтерді тексеріп, қайталап көріңіз.',
  characterCount: '{count} / {max}', dismiss: 'Жабу', notNow: 'Қазір емес', refreshing: 'Жаңартылуда…', requiredLanguage: 'Кемінде бір сөйлейтін тілді таңдаңыз.', errorSessionRefresh: 'Жалғастыру үшін бетті жаңартып, қайта кіріңіз.',
  errorProfileRequired: 'Жалғастыру үшін профиліңізді толтырыңыз.', errorCapacityTooSmall: 'Лимит мақұлданған қатысушылар санынан аз болмауы керек.',
  errorParticipationChanged: 'Өтінім өзгерді. Қатысушылар тізімін жаңартып, қайталап көріңіз.', errorMessageConflict: 'Хабарламаны растау мүмкін болмады. Қайта жіберу алдында әңгімені жаңартыңыз.',
  errorOrganizerCannotLeave: 'Ұйымдастырушы сапардан шығудың орнына одан бас тартуы керек.', errorAccountUnavailable: 'Аккаунт қолжетімсіз. Операторға хабарласыңыз.',
}

const dictionaries = { en, ru, kk }
type Translate = (key: Key, values?: Record<string, string | number>) => string
interface I18n { locale: Locale; setLocale: (locale: Locale) => void; t: Translate }
const I18nContext = createContext<I18n | null>(null)

function savedLocale(): Locale {
  try {
    const saved = localStorage.getItem('bikerskz.locale')
    return saved === 'ru' || saved === 'kk' ? saved : 'en'
  } catch { return 'en' }
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(savedLocale)
  useEffect(() => {
    document.documentElement.lang = locale
    try { localStorage.setItem('bikerskz.locale', locale) } catch { /* Locale persistence is optional. */ }
  }, [locale])
  const t: Translate = (key, values) => Object.entries(values ?? {}).reduce(
    (value, [name, replacement]) => value.replaceAll(`{${name}}`, String(replacement)),
    dictionaries[locale][key] as string,
  )
  return <I18nContext.Provider value={{ locale, setLocale, t }}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const context = useContext(I18nContext)
  if (!context) throw new Error('I18nProvider missing')
  return context
}

export const intlLocale = (locale: Locale) => ({ en: 'en-GB', ru: 'ru-RU', kk: 'kk-KZ' })[locale]

export function errorText(error: unknown, t: Translate) {
  if (!(error instanceof ApiError)) return t('errorGeneric')
  const code = error.code.toLowerCase()
  if (error.status === 0) return t('errorNetwork')
  const specific: Record<string, Key> = { avatar_upload_busy: 'avatarBusy', album_photo_limit: 'albumPhotoLimit', album_video_limit: 'albumVideoLimit', invalid_video_link: 'httpsVideoRequired', announcement_id_conflict: 'announcementConflict', action_id_conflict: 'actionConflict', block_limit: 'blockLimit', push_subscription_limit: 'pushLimit', push_unavailable: 'pushUnavailable', push_not_ready: 'pushNotReady', push_subscription_in_use: 'pushInUse', user_blocked: 'blockedJoining', admin_required: 'errorForbidden', club_image_busy: 'clubImageBusy', photo_too_large: 'photoTooLarge', unsupported_photo: 'photoInvalidType', invalid_photo: 'photoInvalid', invalid_photo_choice: 'photoChoiceInvalid', photo_limit: 'photoLimit', photo_storage_limit: 'photoStorageLimit', photo_upload_expired: 'photoUploadExpired', photo_upload_busy: 'photoUploadBusy', photo_rate_limited: 'photoRateLimited', profile_required: 'errorProfileRequired', capacity_too_small: 'errorCapacityTooSmall', participation_changed: 'errorParticipationChanged', message_id_conflict: 'errorMessageConflict', organizer_cannot_leave: 'errorOrganizerCannotLeave', account_unavailable: 'errorAccountUnavailable', stale_version: 'errorChanges', club_closed: 'errorClubClosed', owner_transfer_required: 'errorClubOwnerLeave', membership_changed: 'errorClubMembership', invalid_owner: 'errorTransferTarget' }
  if (specific[code]) return t(specific[code])
  if (code.includes('full')) return t('errorRideFull')
  if (code.includes('cancelled')) return t('errorCancelled')
  if (code.includes('expir') && (code.includes('code') || code.includes('challenge') || code.includes('otp'))) return t('errorOtpExpired')
  if (code.includes('invalid_code') || code.includes('code_invalid') || code.includes('verification_failed') || code.includes('invalid_otp')) return t('errorOtpInvalid')
  if (code.includes('telegram') || code.includes('provider') || code.includes('delivery') || code === 'verification_unavailable') return t('errorOtpUnavailable')
  if (error.status === 401) return t('errorUnauthorized')
  if (code.includes('csrf') || code === 'invalid_origin') return t('errorSessionRefresh')
  if (error.status === 403) return t('errorForbidden')
  if (error.status === 404) return t('notFound')
  if (error.status === 409) return t('errorConflict')
  if (error.status === 429) return t('errorRateLimit')
  if (error.status === 400 || error.status === 422) return t('errorValidation')
  return t('errorGeneric')
}
