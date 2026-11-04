// Throwaway: three structurally different motorcycle-community directions on /?variant=A|B|C.
// Read-only data and local preview actions; the chosen design must be implemented separately.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ArrowDownIcon as ArrowDown, ArrowLeftIcon as ArrowLeft, ArrowRightIcon as ArrowRight, ArrowUpRightIcon as ArrowUpRight, BellIcon as Bell, CalendarDotsIcon as CalendarDays, CheckIcon as Check, CaretRightIcon as ChevronRight, CompassIcon as Compass, GaugeIcon as Gauge, GlobeHemisphereEastIcon as Globe2, MapPinIcon as MapPin, ListIcon as Menu, MotorcycleIcon as Motorbike, PlusIcon as Plus, PathIcon as Route, MagnifyingGlassIcon as Search, ShieldCheckIcon as ShieldCheck, UsersThreeIcon as Users, WrenchIcon as Wrench, XIcon as X } from '@phosphor-icons/react'
import type { Ride } from '../types'
import './RidesDesignPrototype.css'

type Variant = 'A' | 'B' | 'C'
interface Props { rides: Ride[]; filter: string; onFilterChange: (value: string) => void; loading: boolean; error?: unknown }
interface PreviewMessage { title: string; body: string; ride?: Ride }
interface VariantProps extends Props { samples: boolean; visibleRides: Ride[]; openPreview: (message: PreviewMessage) => void }
const variants: Variant[] = ['A', 'B', 'C']
const names = { A: 'Night Ride', B: 'Rally Roadbook', C: 'Clubhouse' }
const samples: Ride[] = [
  { id: 'sample-almaty', title: 'Almaty morning mountain route', city: 'Almaty', date: '2026-11-07', description: 'Almaty morning mountain route. An easy-paced start, winding roads, and a coffee stop on the way back.', status: 'published' },
  { id: 'sample-astana', title: 'Astana evening loop', city: 'Astana', date: '2026-11-07', description: 'Astana evening loop. A relaxed city ride with time to catch up before heading home.', status: 'published' },
  { id: 'sample-sunday', title: 'Sunday long ride', city: 'Almaty', date: '2026-11-08', description: 'Sunday long ride. Take the longer way, share the road, and make a day of it.', status: 'published' },
]
const photos = {
  ktm: { url: '/bike-models/ktm-390-adventure-2024.png', alt: 'Orange KTM 390 Adventure motorcycle, side view', author: 'KTM AG', source: 'https://commons.wikimedia.org/wiki/File:Ktm-390-adventure-2024.png' },
  tenere: { url: '/bike-models/yamaha-tenere-700-2021.jpg', alt: 'Yamaha Ténéré 700 parked beside a coastal viewpoint', author: 'Happypap55', source: 'https://commons.wikimedia.org/wiki/File:Tenere_700.jpg' },
  enfield: { url: '/bike-models/royal-enfield-interceptor-650-2019.jpg', alt: 'Rider in a leather jacket on an orange Royal Enfield Interceptor 650', author: 'Ofml', source: 'https://commons.wikimedia.org/wiki/File:Royal_Enfield_650_Interceptor.jpg' },
}
function title(ride: Ride) { return ride.title || ride.description || `${ride.city} ride` }
function date(ride: Ride) {
  const value = new Date(`${ride.date}T12:00:00Z`)
  return Number.isNaN(value.getTime()) ? { day: '—', month: 'DATE', full: ride.date } : { day: value.toLocaleDateString('en-GB', { day: '2-digit', timeZone: 'UTC' }), month: value.toLocaleDateString('en-GB', { month: 'short', timeZone: 'UTC' }), full: value.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' }) }
}
function PhotoCredit({ photo, className = '' }: { photo: keyof typeof photos; className?: string }) {
  const value = photos[photo]
  return <figcaption className={`moto-photo-credit ${className}`}>Illustrative catalogue photo · <a href={value.source} target="_blank" rel="noopener noreferrer">{value.author}</a> · <a href="https://creativecommons.org/licenses/by-sa/4.0/" target="_blank" rel="noopener noreferrer">CC BY-SA 4.0</a></figcaption>
}
function SearchBox({ value, onChange, light = false }: { value: string; onChange: (value: string) => void; light?: boolean }) {
  return <label className={`moto-search ${light ? 'moto-search-light' : ''}`}><Search size={19} /><span className="moto-sr-only">Search rides by city or description</span><input type="search" value={value} onChange={(event) => onChange(event.target.value)} placeholder="City, road, or a reason to ride…" />{value && <button aria-label="Clear search" onClick={() => onChange('')} type="button"><X size={17} /></button>}</label>
}
function RideState({ loading, error, empty }: { loading: boolean; error?: unknown; empty: boolean }) {
  return loading ? <p className="moto-state" role="status">Finding your next ride…</p> : error ? <p className="moto-state" role="alert">Ride data is unavailable. This remains a visual preview.</p> : empty ? <p className="moto-state" role="status">No rides match that search. Try another city or clear the search.</p> : null
}
function PreviewTag({ samples: isSample }: { samples: boolean }) { return <span className="moto-data-tag">{isSample ? 'Sample rides · visual only' : 'Live ride data · read only'}</span> }
function previewRide(ride: Ride, isSample: boolean): PreviewMessage {
  return { title: title(ride), body: isSample ? 'This is a sample ride for comparing the designs. No event has been created and no join request will be sent.' : 'This read-only design preview uses your existing ride data. Joining and private ride details remain in the regular application.', ride }
}

export default function RidesDesignPrototype(props: Props) {
  const [params, setParams] = useSearchParams()
  const requested = params.get('variant')
  const variant: Variant = requested === 'B' || requested === 'C' ? requested : 'A'
  const [preview, setPreview] = useState<PreviewMessage | null>(null)
  const isSample = props.rides.length === 0 && !props.loading && !props.error
  const source = isSample ? samples : props.rides
  const visibleRides = source.filter((ride) => `${ride.city} ${ride.description}`.toLocaleLowerCase().includes(props.filter.trim().toLocaleLowerCase()))
  const switchVariant = useCallback((direction: number) => {
    const next = variants[(variants.indexOf(variant) + direction + variants.length) % variants.length]
    const values = new URLSearchParams(params); values.set('variant', next)
    setPreview(null); setParams(values, { replace: true })
  }, [params, setParams, variant])
  useEffect(() => {
    const keydown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || preview) return
      const target = event.target instanceof Element ? event.target : null
      if (target?.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]')) return
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); switchVariant(event.key === 'ArrowRight' ? 1 : -1) }
    }
    window.addEventListener('keydown', keydown)
    return () => window.removeEventListener('keydown', keydown)
  }, [preview, switchVariant])
  const data = { ...props, samples: isSample, visibleRides, openPreview: setPreview }
  return <div className={`design-prototype moto-variant-${variant.toLowerCase()}`}>
    {variant === 'A' ? <VariantA {...data} /> : variant === 'B' ? <VariantB {...data} /> : <VariantC {...data} />}
    <nav className="moto-prototype-switcher" aria-label="Compare design prototypes"><button type="button" onClick={() => switchVariant(-1)} aria-label="Previous design"><ArrowLeft size={20} /></button><div aria-live="polite"><small>DESIGN PREVIEW · {variant} OF A–C</small><strong>{names[variant]}</strong><span>Use ← → to compare</span></div><button type="button" onClick={() => switchVariant(1)} aria-label="Next design"><ArrowRight size={20} /></button></nav>
    {preview && <PreviewDialog message={preview} close={() => setPreview(null)} />}
  </div>
}

function VariantA({ visibleRides, samples: isSample, filter, onFilterChange, loading, error, openPreview }: VariantProps) {
  const explain = (name: string) => openPreview({ title: name, body: 'This control shows the visual direction only. Your existing rides, garage, clubs, and account are unchanged.' })
  return <div className="moto-a-shell">
    <aside className="moto-a-rail"><button className="moto-a-logo" onClick={() => explain('BIKERS.KZ')} aria-label="BIKERS.KZ home">BIKERS<span>.KZ</span><i /></button><span className="moto-a-rail-caption">MADE FOR THE RIDE.</span><nav aria-label="Preview navigation"><button className="moto-a-nav-active" onClick={() => document.getElementById('moto-a-rides')?.scrollIntoView({ behavior: 'smooth' })}><Compass size={20} />Discover<ArrowUpRight size={16} /></button><button onClick={() => explain('Your rides')}><Route size={20} />My rides</button><button onClick={() => explain('Your garage')}><Wrench size={20} />Garage</button><button onClick={() => explain('Your clubs')}><Users size={20} />Clubs</button><button onClick={() => explain('Your inbox')}><Bell size={20} />Inbox</button></nav><div className="moto-a-rail-bottom"><Motorbike size={36} strokeWidth={1.2} /><p>Different bikes.<br />Same direction.</p><span>KAZAKHSTAN <ArrowUpRight size={16} /></span></div><button className="moto-a-account" onClick={() => explain('Rider account')}><span>RK</span><div>Rider account<small>Profile & settings</small></div><ChevronRight size={15} /></button></aside>
    <div className="moto-a-main"><header className="moto-a-top"><span><i /> KAZAKHSTAN’S MOTORCYCLE COMMUNITY</span><div><button onClick={() => explain('Language selection')}><Globe2 size={15} />EN</button><button aria-label="Preview notifications" onClick={() => explain('Notifications')}><Bell size={18} /></button><button className="moto-a-new" onClick={() => explain('Plan a ride')}><Plus size={17} />PLAN A RIDE</button></div></header>
      <section className="moto-a-hero"><div className="moto-a-hero-grid" aria-hidden="true" /><div className="moto-a-hero-copy"><span className="moto-a-kicker"><span /> LESS SCROLLING. MORE RIDING.</span><h1>FIND YOUR<br /><em>NEXT RIDE.</em></h1><p>City streets. Mountain turns. The long way home.<br />Find a plan. Meet your people. Ride together.</p><button className="moto-a-primary" onClick={() => document.getElementById('moto-a-rides')?.scrollIntoView({ behavior: 'smooth' })}>EXPLORE RIDES<ArrowDown size={18} /></button></div><div className="moto-a-hero-number" aria-hidden="true">KZ</div><figure className="moto-a-bike"><img src={photos.ktm.url} alt={photos.ktm.alt} /><PhotoCredit photo="ktm" /></figure><div className="moto-a-hero-foot"><span><ShieldCheck size={14} /> Ride plans built around organizer approval</span><span>ALL BIKES. ALL RIDERS. <ArrowUpRight size={17} /></span></div></section>
      <section className="moto-a-content" id="moto-a-rides"><div className="moto-a-rides-column"><div className="moto-a-section-top"><div><span className="moto-a-overline">GET OUT THERE</span><h2>UPCOMING RIDES<span>↗</span></h2></div><PreviewTag samples={isSample} /></div><SearchBox value={filter} onChange={onFilterChange} /><RideState loading={loading} error={error} empty={!visibleRides.length} /><div className="moto-a-ride-list">{visibleRides.map((ride, index) => { const when = date(ride); return <button className="moto-a-ride-row" key={ride.id} onClick={() => openPreview(previewRide(ride, isSample))}><span className="moto-a-date"><strong>{when.day}</strong><small>{when.month}</small></span><span className="moto-a-ride-text"><span><MapPin size={12} />{ride.city}<i />{isSample ? ['MOUNTAIN ROADS', 'CITY LOOP', 'LONG WAY HOME'][index % 3] : 'PLANNED RIDE'}</span><strong>{title(ride)}</strong><small><ShieldCheck size={12} />Organizer approval · {isSample ? 'Sample preview' : 'Private details stay with the group'}</small></span><ArrowUpRight size={22} /></button> })}</div></div><aside className="moto-a-plan-panel"><span className="moto-a-mini-mark"><Route size={23} /></span><span className="moto-a-overline">A GOOD RIDE STARTS WITH A PLAN.</span><h3>BRING<br />YOUR<br /><em>PEOPLE.</em></h3><p>You choose the route.<br />Your riders bring the stories.</p><button onClick={() => explain('Plan your next ride')}>PLAN A RIDE<ArrowUpRight size={17} /></button><span className="moto-a-panel-bottom">ONE PLAN. ONE RIDE ROOM.</span></aside></section>
      <footer className="moto-a-footer"><span>BIKERS.KZ <i /> BUILT AROUND MOTORCYCLES.</span><span>THROWAWAY DESIGN PROTOTYPE · NO ACTIONS ARE SAVED</span></footer>
    </div>
  </div>
}

function VariantB({ visibleRides, samples: isSample, filter, onFilterChange, loading, error, openPreview }: VariantProps) {
  const explain = (name: string) => openPreview({ title: name, body: 'This is a read-only design concept. Planning, joining, and account changes are not performed in the preview.' })
  return <div className="moto-b-shell"><header className="moto-b-top"><button className="moto-b-wordmark" onClick={() => explain('BIKERS.KZ')}><span><Motorbike size={28} /></span>BIKERS.KZ</button><nav aria-label="Preview navigation"><button className="moto-b-active" onClick={() => document.getElementById('moto-b-rides')?.scrollIntoView({ behavior: 'smooth' })}>The ride board</button><button onClick={() => explain('Garage')}>The garage</button><button onClick={() => explain('Clubs')}>The community</button></nav><button className="moto-b-new" onClick={() => explain('Plan a ride')}><Plus size={16} />Plan a ride<ArrowUpRight size={16} /></button><button className="moto-b-mobile-menu" aria-label="Preview navigation menu" onClick={() => explain('Navigation')}><Menu size={23} /></button></header>
    <div className="moto-b-main"><div className="moto-b-edition"><span><i /> KAZAKHSTAN · ON TWO WHEELS</span><span>THE RIDE BOARD / DESIGN STUDY B</span></div><section className="moto-b-heading"><div><h1>GOOD ROADS.<br />BETTER COMPANY<span>.</span></h1><p>A place for the plans that get you out the door.<br />Find a ride, bring your bike, and take the long way.</p></div><div className="moto-b-stamp"><Route size={29} /><span>THE ROUTE<br />IS BETTER<br /><strong>TOGETHER.</strong></span></div></section>
      <div className="moto-b-layout"><section className="moto-b-roadbook" id="moto-b-rides"><div className="moto-b-list-heading"><span><i /> FIND YOUR NEXT OUTING</span><PreviewTag samples={isSample} /></div><SearchBox value={filter} onChange={onFilterChange} light /><div className="moto-b-table-head"><span>DEPARTURES</span><span>ROADBOOK</span><ArrowDown size={16} /></div><RideState loading={loading} error={error} empty={!visibleRides.length} /><div className="moto-b-itinerary">{visibleRides.map((ride, index) => { const when = date(ride); return <button className="moto-b-stop" key={ride.id} onClick={() => openPreview(previewRide(ride, isSample))}><span className="moto-b-stop-number">{String(index + 1).padStart(2, '0')}</span><span className="moto-b-stop-copy"><span><MapPin size={13} />{ride.city}<i />{when.full}</span><strong>{title(ride)}</strong><small>{ride.description}</small><span className="moto-b-stop-policy"><ShieldCheck size={13} />Organizer-approved group {isSample && '· Sample ride'}</span></span><ArrowUpRight size={23} /></button> })}</div><div className="moto-b-roadbook-note"><span><Check size={18} /></span><p>A public invitation. A private plan.<br /><strong>The exact meeting point opens after organizer approval.</strong></p></div></section><aside className="moto-b-feature"><figure><img src={photos.tenere.url} alt={photos.tenere.alt} /><div className="moto-b-photo-label">BEYOND THE EVERYDAY<ArrowUpRight size={19} /></div><div className="moto-b-photo-copy"><span>THE SPIRIT OF THE RIDE</span><h2>TAKE THE<br />LONG WAY.</h2></div><PhotoCredit photo="tenere" /></figure><div className="moto-b-feature-foot"><span>ANY MAKE.<br /><strong>ONE COMMUNITY.</strong></span><Motorbike size={42} strokeWidth={1.2} /></div></aside></div>
      <section className="moto-b-principles"><div><span>01 / THE PLAN</span><strong>A clear place to start.</strong><p>The ride, the route, and the people. Together.</p></div><div><span>02 / THE GARAGE</span><strong>Every bike has a story.</strong><p>Keep your motorcycles and maintenance close.</p></div><div><span>03 / THE PEOPLE</span><strong>Your kind of company.</strong><p>Find a club. Turn introductions into rides.</p></div></section><footer className="moto-b-footer"><strong>BIKERS.KZ</strong><span>DESIGN PROTOTYPE · SAMPLE RIDES ARE ILLUSTRATIVE · NOTHING IS SAVED</span><ArrowUpRight size={23} /></footer>
    </div></div>
}

function VariantC({ visibleRides, samples: isSample, filter, onFilterChange, loading, error, openPreview }: VariantProps) {
  const explain = (name: string) => openPreview({ title: name, body: 'This visual preview does not create a ride, join a club, or publish a post. Sample club labels show the proposed layout only.' })
  const first = visibleRides[0]
  return <div className="moto-c-shell"><header className="moto-c-top"><button className="moto-c-wordmark" onClick={() => explain('BIKERS.KZ')}>BIKERS<span>.KZ</span><Motorbike size={28} /></button><nav aria-label="Preview navigation"><button className="moto-c-nav-active" onClick={() => document.getElementById('moto-c-feed')?.scrollIntoView({ behavior: 'smooth' })}>Ride board</button><button onClick={() => explain('Clubs')}>Clubs</button><button onClick={() => explain('Garage')}>Garage</button></nav><div><button className="moto-c-notifications" aria-label="Preview inbox" onClick={() => explain('Inbox')}><Bell size={20} /></button><button className="moto-c-account" aria-label="Preview rider profile" onClick={() => explain('Rider profile')}><Gauge size={21} /></button><button className="moto-c-plan" onClick={() => explain('Plan a ride')}><Plus size={17} />Plan a ride</button></div></header>
    <div className="moto-c-main"><section className="moto-c-heading"><div><span><i /> MOTORCYCLES BRING US TOGETHER</span><h1>THE RIDE STARTS<br />WITH <em>YOUR PEOPLE.</em></h1></div><p>Find a plan worth joining. <br />Make a few introductions. <br />Let the road do the rest.</p></section><div className="moto-c-layout"><section className="moto-c-feed" id="moto-c-feed"><div className="moto-c-feed-toolbar"><h2>On the ride board</h2><PreviewTag samples={isSample} /></div><SearchBox value={filter} onChange={onFilterChange} /><RideState loading={loading} error={error} empty={!visibleRides.length} />{first && <article className="moto-c-lead-post"><div className="moto-c-post-header"><span className="moto-c-author-symbol"><Motorbike size={24} /></span><div><strong>{isSample ? 'A little inspiration for the weekend' : 'From the ride board'}</strong><span><MapPin size={12} />{first.city} · {date(first).full}{isSample && ' · Sample ride'}</span></div><span className="moto-c-post-label">LET’S RIDE</span></div><figure className="moto-c-post-photo"><img src={photos.enfield.url} alt={photos.enfield.alt} /><div><span>DIFFERENT BIKES.</span><strong>SAME FEELING.</strong></div><PhotoCredit photo="enfield" /></figure><div className="moto-c-post-body"><span className="moto-c-city"><MapPin size={13} />{first.city}<i />PLANNED RIDE</span><h3>{title(first)}</h3><p>{first.description}</p><div className="moto-c-post-actions"><span><ShieldCheck size={14} />Organizer-approved group</span><button onClick={() => openPreview(previewRide(first, isSample))}>View the plan<ArrowUpRight size={18} /></button></div></div></article>}{visibleRides.slice(1).map((ride, index) => <article className="moto-c-follow-post" key={ride.id}>{index === 0 ? <figure><img src={photos.tenere.url} alt={photos.tenere.alt} /><PhotoCredit photo="tenere" /></figure> : <div className="moto-c-route-illustration" aria-hidden="true"><Route size={76} strokeWidth={1} /><span>THE LONG WAY HOME</span></div>}<div className="moto-c-follow-copy"><span className="moto-c-city"><MapPin size={12} />{ride.city} · {date(ride).full}</span><h3>{title(ride)}</h3><p>{ride.description}</p><button onClick={() => openPreview(previewRide(ride, isSample))}>See this ride<ArrowUpRight size={17} /></button>{isSample && <small>Sample ride · visual only</small>}</div></article>)}</section>
      <aside className="moto-c-community-rail"><section className="moto-c-club-panel"><span className="moto-c-rail-kicker">FIND YOUR CIRCLE</span><h2>GOOD COMPANY.<br /><em>GREAT ROADS.</em></h2><p>The best part of a club?<br />There’s always a next ride.</p><span className="moto-c-sample-label">SAMPLE CLUB LABELS · DESIGN ONLY</span><button className="moto-c-club-row" onClick={() => explain('Almaty road crew — sample club')}><span>AR<span>↗</span></span><div><strong>Almaty road crew</strong><small>Road rides & good coffee</small></div><ChevronRight size={18} /></button><button className="moto-c-club-row" onClick={() => explain('Weekend wanderers — sample club')}><span>WW<span>↗</span></span><div><strong>Weekend wanderers</strong><small>Out of town, together</small></div><ChevronRight size={18} /></button><button className="moto-c-club-all" onClick={() => explain('Explore clubs')}>Explore clubs<ArrowRight size={18} /></button></section><section className="moto-c-garage-panel"><span className="moto-c-rail-kicker">THE OTHER HALF OF YOUR STORY</span><Motorbike size={49} strokeWidth={1.2} /><h3>YOUR BIKE.<br />YOUR GARAGE.</h3><p>Keep the bikes you love<br />ready for the roads ahead.</p><button onClick={() => explain('Your garage')}>Open your garage<ArrowUpRight size={17} /></button></section><div className="moto-c-rail-footer"><ShieldCheck size={19} /><p>Public invitations.<br />Private meeting details.<br />Your ride, your circle.</p></div></aside></div><footer className="moto-c-footer"><span>BIKERS.KZ · KAZAKHSTAN, TOGETHER.</span><span>READ-ONLY DESIGN PROTOTYPE · NO CHANGES ARE SAVED</span></footer></div></div>
}

function PreviewDialog({ message, close }: { message: PreviewMessage; close: () => void }) {
  const panel = useRef<HTMLDivElement>(null)
  const closeButton = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    closeButton.current?.focus()
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close()
      if (event.key === 'Tab') {
        const elements = panel.current?.querySelectorAll<HTMLButtonElement>('button')
        const first = elements?.[0]; const last = elements?.[elements.length - 1]
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
      }
    }
    document.addEventListener('keydown', keydown)
    return () => { document.removeEventListener('keydown', keydown); previous?.focus() }
  }, [close])
  return <div className="moto-preview-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) close() }}><div className="moto-preview-dialog" ref={panel} role="dialog" aria-modal="true" aria-labelledby="moto-preview-title"><button ref={closeButton} className="moto-preview-close" aria-label="Close preview details" onClick={close}><X size={22} /></button><span><Motorbike size={28} />VISUAL PREVIEW</span><h2 id="moto-preview-title">{message.title}</h2><p>{message.body}</p>{message.ride && <div className="moto-preview-ride-details"><span><MapPin size={16} />{message.ride.city}</span><span><CalendarDays size={16} />{date(message.ride).full}</span><p>{message.ride.description}</p><small><ShieldCheck size={15} />Exact meeting details and participation stay in the regular ride flow.</small></div>}<button className="moto-preview-return" onClick={close}>Back to the design<ArrowRight size={17} /></button></div></div>
}
