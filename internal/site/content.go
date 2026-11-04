package site

type service struct {
	Title string
	Text  string
}

type category struct {
	Names    []string
	Taglines []string
	About    []string
	Services []service
	Values   []service
}

type langPack struct {
	Prefixes   []string
	Cities     []string
	Hours      []string
	Categories []category
	UI         map[string]string
}

var packs = map[string]langPack{
	"ru": {
		Prefixes: []string{"Северный", "Вектор", "Меридиан", "Горизонт", "Орион", "Кедр", "Полярис", "Стрела", "Ладога", "Алтай", "Сфера", "Прайм", "Берег", "Перспектива", "Основа", "Гранит"},
		Cities:   []string{"Москва", "Санкт-Петербург", "Казань", "Екатеринбург", "Новосибирск", "Нижний Новгород", "Самара", "Краснодар", "Пермь", "Воронеж"},
		Hours:    []string{"Пн–Пт, 9:00–18:00", "Пн–Пт, 10:00–19:00", "Пн–Сб, 9:00–20:00", "Ежедневно, 10:00–20:00"},
		UI: map[string]string{
			"services": "Услуги", "about": "О компании", "why": "Почему мы", "contact": "Связаться с нами",
			"name": "Ваше имя", "phone_or_email": "Телефон или e-mail", "message": "Сообщение", "send": "Отправить",
			"hours": "Часы работы", "city": "Город", "rights": "Все права защищены", "cta": "Оставить заявку",
			"thanks_title": "Спасибо!", "thanks": "Мы получили вашу заявку и свяжемся с вами в ближайшее время.", "back": "На главную",
			"notfound": "Страница не найдена", "signin": "Вход", "login": "Логин", "password": "Пароль", "enter": "Войти",
			"signin_hint": "Вход для сотрудников", "bad_login": "Неверный логин или пароль", "too_many": "Слишком много попыток, попробуйте позже",
		},
		Categories: []category{
			{
				Names:    []string{"Консалтинг", "Бизнес-Консалт", "Партнёры"},
				Taglines: []string{"Помогаем бизнесу расти устойчиво", "Стратегия, финансы и процессы — в одних руках", "Решения, которые работают на результат"},
				About:    []string{"Мы консультируем малый и средний бизнес по вопросам стратегии, управленческого учёта и операционной эффективности. Работаем проектно и на абонентской основе, всегда фиксируем измеримый результат.", "Наша команда — практики с опытом работы в производстве, торговле и сфере услуг. Мы не продаём шаблоны: каждый проект начинается с диагностики и заканчивается внедрением."},
				Services: []service{{"Стратегия развития", "Анализ рынка, позиционирование и план роста на 1–3 года."}, {"Управленческий учёт", "Постановка отчётности, бюджетирование и контроль денежных потоков."}, {"Оптимизация процессов", "Описание и перестройка бизнес-процессов, снижение издержек."}, {"Сопровождение сделок", "Due diligence, оценка бизнеса и подготовка к переговорам."}, {"Обучение руководителей", "Практические программы для собственников и менеджеров."}},
				Values:   []service{{"Опыт", "Более 10 лет практики в разных отраслях."}, {"Прозрачность", "Понятные этапы, сроки и стоимость работ."}, {"Результат", "Фиксируем KPI проекта до начала работ."}},
			},
			{
				Names:    []string{"Веб-Студия", "Диджитал", "Лаб"},
				Taglines: []string{"Создаём сайты и сервисы, которые приносят клиентов", "Дизайн и разработка цифровых продуктов", "От идеи до запуска — быстро и аккуратно"},
				About:    []string{"Мы небольшая команда дизайнеров и разработчиков. Делаем корпоративные сайты, интернет-магазины и внутренние сервисы, а после запуска поддерживаем и развиваем проекты.", "Работаем по прозрачному процессу: исследование, прототип, дизайн, разработка, тестирование и запуск. Клиент видит прогресс каждую неделю."},
				Services: []service{{"Корпоративные сайты", "Современные адаптивные сайты с удобной системой управления."}, {"Интернет-магазины", "Каталог, корзина, оплата и интеграция с учётными системами."}, {"UX/UI-дизайн", "Исследования, прототипы и дизайн-системы."}, {"Техническая поддержка", "Обновления, резервные копии и мониторинг 24/7."}, {"Интеграции", "Связываем сайт с CRM, 1С и сервисами доставки."}},
				Values:   []service{{"Сроки", "Соблюдаем договорённости и календарный план."}, {"Качество", "Код-ревью, тестирование и документация."}, {"Поддержка", "Остаёмся на связи после запуска."}},
			},
			{
				Names:    []string{"Логистик", "Транс", "Карго"},
				Taglines: []string{"Надёжная доставка грузов по России и СНГ", "Логистика без лишних забот", "Перевозим, храним, доставляем вовремя"},
				About:    []string{"Организуем перевозки сборных и целых грузов автомобильным и железнодорожным транспортом. Берём на себя оформление документов, страхование и отслеживание.", "Собственный склад и проверенные перевозчики позволяют нам предлагать стабильные сроки и честные тарифы для компаний любого размера."},
				Services: []service{{"Автоперевозки", "Доставка сборных и выделенных грузов."}, {"Складское хранение", "Ответственное хранение и обработка заказов."}, {"Таможенное оформление", "Подготовка документов и сопровождение."}, {"Экспедирование", "Контроль погрузки, маршрута и выгрузки."}, {"Страхование грузов", "Защита груза на всём пути следования."}},
				Values:   []service{{"Пунктуальность", "Доставка в согласованное окно."}, {"Контроль", "Статус груза на каждом этапе."}, {"Гибкость", "Подбираем маршрут под задачу."}},
			},
			{
				Names:    []string{"Инжиниринг", "Проект", "Техсервис"},
				Taglines: []string{"Проектирование и монтаж инженерных систем", "Инженерные решения для зданий и производств", "Надёжная инженерия под ключ"},
				About:    []string{"Проектируем и монтируем системы отопления, вентиляции, кондиционирования и электроснабжения для коммерческих и производственных объектов.", "В штате — инженеры-проектировщики и монтажные бригады с допусками СРО. Даём гарантию на выполненные работы и обслуживаем объекты после сдачи."},
				Services: []service{{"Проектирование", "Разработка проектной и рабочей документации."}, {"Вентиляция и кондиционирование", "Подбор оборудования, монтаж и пусконаладка."}, {"Электроснабжение", "Силовые сети, освещение, щитовое оборудование."}, {"Отопление и водоснабжение", "Котельные, тепловые пункты, внутренние сети."}, {"Сервисное обслуживание", "Плановое ТО и аварийные выезды."}},
				Values:   []service{{"Допуски", "Членство в СРО и все необходимые лицензии."}, {"Гарантия", "До 3 лет на смонтированные системы."}, {"Сроки", "Работаем по утверждённому графику."}},
			},
			{
				Names:    []string{"Бухгалтерия", "Финанс", "Аудит"},
				Taglines: []string{"Бухгалтерия, в которой можно быть уверенным", "Учёт и налоги — наша забота, бизнес — ваша", "Аккуратный учёт для малого бизнеса"},
				About:    []string{"Ведём бухгалтерский и налоговый учёт для ИП и ООО на всех системах налогообложения. Сдаём отчётность вовремя и отвечаем за результат по договору.", "Каждому клиенту назначаем персонального бухгалтера. Консультируем по налоговой нагрузке и помогаем при проверках."},
				Services: []service{{"Бухгалтерское обслуживание", "Полное ведение учёта и отчётности."}, {"Расчёт заработной платы", "Кадровый учёт, зарплата и взносы."}, {"Налоговое планирование", "Законная оптимизация налоговой нагрузки."}, {"Восстановление учёта", "Приводим в порядок учёт за прошлые периоды."}, {"Сопровождение проверок", "Подготовка документов и защита интересов."}},
				Values:   []service{{"Ответственность", "Финансовая ответственность по договору."}, {"Конфиденциальность", "Надёжная защита ваших данных."}, {"Доступность", "Отвечаем на вопросы в течение дня."}},
			},
		},
	},
	"en": {
		Prefixes: []string{"Northfield", "Bluestone", "Clearpath", "Silverline", "Brightwave", "Oakridge", "Pinecrest", "Redwood", "Summit", "Harbor", "Westbrook", "Ironleaf", "Granite", "Meridian", "Lakeside", "Evergreen"},
		Cities:   []string{"Amsterdam", "Berlin", "Dublin", "Helsinki", "Lisbon", "Prague", "Tallinn", "Vienna", "Warsaw", "Zurich"},
		Hours:    []string{"Mon–Fri, 9:00–17:00", "Mon–Fri, 8:30–18:00", "Mon–Sat, 9:00–19:00"},
		UI: map[string]string{
			"services": "Services", "about": "About us", "why": "Why us", "contact": "Get in touch",
			"name": "Your name", "phone_or_email": "Phone or e-mail", "message": "Message", "send": "Send",
			"hours": "Office hours", "city": "City", "rights": "All rights reserved", "cta": "Contact us",
			"thanks_title": "Thank you!", "thanks": "We have received your message and will get back to you shortly.", "back": "Back to home",
			"notfound": "Page not found", "signin": "Sign in", "login": "Username", "password": "Password", "enter": "Sign in",
			"signin_hint": "Staff access", "bad_login": "Invalid username or password", "too_many": "Too many attempts, try again later",
		},
		Categories: []category{
			{
				Names:    []string{"Consulting", "Advisory", "Partners"},
				Taglines: []string{"Helping businesses grow with confidence", "Strategy, finance and operations under one roof", "Practical advice that delivers results"},
				About:    []string{"We advise small and mid-sized companies on strategy, management accounting and operational efficiency. Every engagement starts with a diagnosis and ends with measurable outcomes.", "Our consultants have hands-on experience in manufacturing, retail and services. We do not sell templates — we work alongside your team until the change sticks."},
				Services: []service{{"Growth strategy", "Market analysis, positioning and a 1–3 year growth plan."}, {"Management reporting", "Budgeting, KPIs and cash-flow control."}, {"Process improvement", "Mapping and redesigning processes to cut costs."}, {"Transaction support", "Due diligence, valuation and negotiation prep."}, {"Leadership training", "Practical programs for owners and managers."}},
				Values:   []service{{"Experience", "Over a decade of cross-industry practice."}, {"Transparency", "Clear stages, timelines and pricing."}, {"Outcomes", "KPIs agreed before work begins."}},
			},
			{
				Names:    []string{"Studio", "Digital", "Labs"},
				Taglines: []string{"Websites and products that win customers", "Design and engineering for the web", "From idea to launch, done right"},
				About:    []string{"We are a small team of designers and engineers building company websites, online stores and internal tools — and supporting them long after launch.", "Our process is simple and transparent: research, prototype, design, build, test and launch, with weekly demos along the way."},
				Services: []service{{"Company websites", "Fast, responsive sites with an easy CMS."}, {"E-commerce", "Catalogs, checkout and payment integrations."}, {"UX/UI design", "Research, prototypes and design systems."}, {"Maintenance", "Updates, backups and 24/7 monitoring."}, {"Integrations", "Connecting your site to CRM and logistics."}},
				Values:   []service{{"On time", "We plan carefully and keep our promises."}, {"Quality", "Code review, testing and documentation."}, {"Support", "We stay with you after launch."}},
			},
			{
				Names:    []string{"Logistics", "Freight", "Cargo"},
				Taglines: []string{"Reliable freight across Europe", "Logistics without the headache", "We move, store and deliver on time"},
				About:    []string{"We arrange full and part-load road and rail freight, handling paperwork, insurance and tracking for our clients.", "Our own warehouse and a network of vetted carriers let us offer predictable lead times and fair rates for companies of any size."},
				Services: []service{{"Road freight", "Full and groupage shipments."}, {"Warehousing", "Storage, picking and fulfilment."}, {"Customs brokerage", "Documentation and clearance support."}, {"Freight forwarding", "End-to-end shipment management."}, {"Cargo insurance", "Protection for every leg of the journey."}},
				Values:   []service{{"Punctuality", "Delivery within the agreed window."}, {"Visibility", "Status updates at every stage."}, {"Flexibility", "Routes tailored to your needs."}},
			},
			{
				Names:    []string{"Engineering", "Technical", "Systems"},
				Taglines: []string{"Building services, designed and installed", "Engineering for commercial and industrial sites", "Dependable engineering, turnkey"},
				About:    []string{"We design and install heating, ventilation, air conditioning and electrical systems for commercial and industrial buildings.", "Our in-house engineers and certified installation crews deliver to schedule, and we maintain every system we build."},
				Services: []service{{"Design", "Concept, detailed and as-built documentation."}, {"HVAC", "Equipment selection, installation and commissioning."}, {"Electrical", "Power distribution, lighting and switchgear."}, {"Plumbing & heating", "Boiler rooms and internal networks."}, {"Maintenance", "Planned servicing and emergency call-outs."}},
				Values:   []service{{"Certified", "Fully licensed and insured."}, {"Warranty", "Up to 3 years on installed systems."}, {"Schedule", "We work to an agreed programme."}},
			},
			{
				Names:    []string{"Accounting", "Finance", "Bookkeeping"},
				Taglines: []string{"Accounting you can rely on", "We handle the numbers, you run the business", "Accurate books for growing companies"},
				About:    []string{"We provide bookkeeping, payroll and tax compliance for sole traders and limited companies, filing every return on time.", "Each client gets a dedicated accountant who knows their business and is available when questions come up."},
				Services: []service{{"Bookkeeping", "Complete, accurate and up-to-date records."}, {"Payroll", "Salaries, contributions and payslips."}, {"Tax planning", "Legal and efficient tax structuring."}, {"Catch-up accounting", "Getting past periods in order."}, {"Audit support", "Preparation and representation."}},
				Values:   []service{{"Accountability", "Our liability is written into the contract."}, {"Confidentiality", "Your data is kept secure."}, {"Responsiveness", "Answers within one business day."}},
			},
		},
	},
}
