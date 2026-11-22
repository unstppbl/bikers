// The create buttons in the top bar and the tab bar show newRideShort. newRideLabel is their accessible name and must contain that text.
// backToMe leads back to the Me page from the pages reached from it; back ("Back to rides") is for links to the rides feed.
// An edit form leads back to the page it edits: backToRide, backToClub and backToBike.
export const shellEn = {
  navMain: 'Main navigation', navTabs: 'Sections', tabMe: 'Me', tabGarage: 'Garage', newRideShort: 'New ride', newRideLabel: 'New ride',
  meGuestTitle: 'Your space', meGuestText: 'Sign in with Telegram to join rides, keep a garage and follow your clubs.',
  meEditProfile: 'Edit profile', meHubLabel: 'Your space and settings', backToMe: 'Back to your space',
  backToRide: 'Back to ride', backToClub: 'Back to club', backToBike: 'Back to motorcycle',
  ridesFeedText: 'Ask to join. The organizer shares the plan once you’re in.',
  planAfterApproval: 'Plan after approval', allDates: 'Any date', rideCoverAlt: 'Ride in {city}', cityPhotoAlt: 'Photo of {city}',
  planTitle: 'The plan', planLocked: 'Shared after approval', youAreGoing: 'You’re going',
}

export const shellRu: Record<keyof typeof shellEn, string> = {
  navMain: 'Основная навигация', navTabs: 'Разделы', tabMe: 'Я', tabGarage: 'Гараж', newRideShort: 'Создать', newRideLabel: 'Создать поездку',
  meGuestTitle: 'Ваше пространство', meGuestText: 'Войдите через Telegram, чтобы присоединяться к поездкам, вести гараж и следить за клубами.',
  meEditProfile: 'Изменить профиль', meHubLabel: 'Ваше пространство и настройки', backToMe: 'К вашему пространству',
  backToRide: 'К поездке', backToClub: 'К клубу', backToBike: 'К мотоциклу',
  ridesFeedText: 'Попроситесь в поездку. Организатор откроет план, когда вас одобрит.',
  planAfterApproval: 'План после одобрения', allDates: 'Любая дата', rideCoverAlt: 'Поездка, {city}', cityPhotoAlt: 'Фото: {city}',
  planTitle: 'План', planLocked: 'Откроется после одобрения', youAreGoing: 'Вы едете',
}

export const shellKk: Record<keyof typeof shellEn, string> = {
  navMain: 'Негізгі навигация', navTabs: 'Бөлімдер', tabMe: 'Мен', tabGarage: 'Гараж', newRideShort: 'Құру', newRideLabel: 'Сапар құру',
  meGuestTitle: 'Жеке кеңістік', meGuestText: 'Сапарларға қосылу, гараж жүргізу және клубтарды бақылау үшін Telegram арқылы кіріңіз.',
  meEditProfile: 'Профильді өзгерту', meHubLabel: 'Жеке кеңістік пен баптаулар', backToMe: 'Жеке кеңістікке оралу',
  backToRide: 'Сапарға оралу', backToClub: 'Клубқа оралу', backToBike: 'Мотоциклге оралу',
  ridesFeedText: 'Қосылуға сұраныс жіберіңіз. Ұйымдастырушы мақұлдаған соң жоспарды ашады.',
  planAfterApproval: 'Жоспар мақұлданғаннан кейін', allDates: 'Кез келген күн', rideCoverAlt: 'Сапар, {city}', cityPhotoAlt: '{city} қаласының суреті',
  planTitle: 'Жоспар', planLocked: 'Мақұлданғаннан кейін ашылады', youAreGoing: 'Сіз барасыз',
}
