import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowRightIcon as ArrowRight, CalendarDotsIcon as CalendarDays, CheckIcon as Check, CaretRightIcon as ChevronRight, ClockIcon as Clock3, LockKeyIcon as LockKeyhole, MapPinIcon as MapPin, ChatCircleIcon as MessageCircle, MotorcycleIcon as Motorbike, PlayIcon as Play, ArrowCounterClockwiseIcon as RotateCcw } from '@phosphor-icons/react'
import type { Locale } from '../../types'
import { Stage } from './kit/stage'
import type { Act } from './kit/act'
import { Window, Bar } from './kit/window'
import { Row } from './kit/parts/Row'
import { States } from './kit/motion/States'
import { Badge } from './kit/controls/Badge'
import { Bubble, Thread } from './kit/messages'
import { useReducedMotion } from './kit/reduced-motion'
import { rideDemoCopy } from './copy'
import './ride-demo.css'

type Scene = 'board' | 'preview' | 'pending' | 'approved' | 'room'
type Story = { scene: Scene }
const initial: Story = { scene: 'board' }
const poster: Story = { scene: 'room' }

// Starting point: Stageshow dark-hero's compact window and share's measured
// click→state causality. This authored illustration never reads or mutates API data.
async function script(a: Act<Story>) {
  await a.wait(1100)
  a.mark('select-public-preview')
  await a.click({ at: 'ride', x: 0.89, y: 0.48 })
  await a.set({ scene: 'preview' })
  await a.wait(1350)
  a.mark('request-place')
  await a.click({ at: 'request', x: 0.88, y: 0.55 })
  await a.set({ scene: 'pending' })
  await a.move({ at: 'rest', x: 0.87, y: 0.5 }, { duration: 400 })
  await a.wait(1900)
  a.mark('organizer-approves')
  await a.set({ scene: 'approved' })
  await a.wait(1650)
  a.mark('open-private-room')
  await a.click({ at: 'room', x: 0.9, y: 0.55 })
  await a.set({ scene: 'room' })
  await a.move({ at: 'rest', x: 0.87, y: 0.5 }, { duration: 450 })
  a.mark('payoff')
  await a.wait(2200)
}

export function BikersRideDemo({ locale }: { locale: Locale }) {
  const [run, setRun] = useState(0)
  const [started, setStarted] = useState(false)
  const reducedMotion = useReducedMotion()
  const restartTimer = useRef<number | undefined>(undefined)
  useEffect(() => () => { if (restartTimer.current) window.clearTimeout(restartTimer.current) }, [])
  if (reducedMotion && started) setStarted(false)
  const [restarting, setRestarting] = useState(false)
  const copy = rideDemoCopy[locale]
  const replay = useCallback(() => {
    if (reducedMotion) return
    setRestarting(true)
    // The real Replay control causes the only rewind; fade the old story first.
    restartTimer.current = window.setTimeout(() => { setStarted(true); setRun((value) => value + 1); setRestarting(false) }, 180)
  }, [reducedMotion])
  return <figure className="bikers-ride-demo" aria-label={copy.example}>
    <div className="brd-label"><Motorbike size={16} strokeWidth={2} /><span>{copy.example}</span></div>
    <div className="brd-story" data-restarting={restarting || undefined} aria-hidden="true">
      <RideDemoScene key={`${locale}:${run}`} locale={locale} play={started && !reducedMotion} />
    </div>
    <figcaption className="brd-controls"><span className="sr-only">{copy.summary}</span><span className="brd-static-caption" hidden={!reducedMotion}>{copy.static}</span>{!reducedMotion && <button type="button" onClick={replay} disabled={restarting}>{started ? <RotateCcw size={16} strokeWidth={2} /> : <Play size={16} strokeWidth={2} />}{started ? copy.replay : copy.play}</button>}</figcaption>
  </figure>
}


// Exported separately for the isolated authored-scene rehearsal, without app data.
export function RideDemoScene({ locale, play = false }: { locale: Locale; play?: boolean }) {
  const copy = rideDemoCopy[locale]
  return <Stage<Story> width={358} height={342} compact={{ below: 344, width: 320, height: 308 }} theme="dark" fidelity="standard" play="once" initial={initial} poster={poster} script={play ? script : undefined} className="brd-stage" cursor={play ? { at: { at: 'ride', x: 0.89, y: 0.48 }, style: 'arrow', ripple: true, idle: 650 } : undefined} tokens={{
        '--ss-canvas': 'var(--surface)', '--ss-surface': 'var(--surface)', '--ss-chrome': 'var(--surface-raised)', '--ss-raised': 'var(--surface-raised)', '--ss-well': 'var(--chrome)', '--ss-text-strong': 'var(--text)', '--ss-text': 'var(--text-muted)', '--ss-text-faint': 'var(--text-subtle)', '--ss-icon': 'var(--text-muted)', '--ss-icon-faint': 'var(--text-subtle)', '--ss-line': 'var(--border)', '--ss-line-faint': 'var(--border)', '--ss-accent': 'var(--accent)', '--ss-accent-2': 'var(--accent)', '--ss-accent-text': 'var(--accent)', '--ss-accent-soft': 'var(--accent-surface)', '--ss-success': 'var(--success)', '--ss-success-soft': 'var(--success-surface)', '--ss-success-text': 'var(--success)', '--ss-warning': 'var(--warning)', '--ss-warning-soft': 'var(--warning-surface)', '--ss-warning-text': 'var(--warning)', '--ss-font': 'var(--font-body)', '--ss-radius-window': 'var(--radius-lg)', '--ss-radius-lg': 'var(--radius-md)', '--ss-radius-md': 'var(--radius-sm)', '--ss-ink-1': 'var(--surface-raised)', '--ss-ink-2': 'var(--surface-hover)', '--ss-ink-3': 'var(--border)', '--ss-ink-4': 'var(--border-strong)', '--ss-ink-5': 'var(--text-subtle)',
      }}>
        {(state) => <Window url="bikers.kz" toolbar={<span className="brd-example">{copy.exampleShort}</span>} lights="grey" className="brd-window"><div className="brd-body">
          <div className="brd-status-line" data-target="rest"><States value={state.scene === 'room' ? 'room' : state.scene === 'board' ? 'board' : 'preview'} align="start" states={{ board: <span>{copy.board}</span>, preview: <span>{copy.preview}</span>, room: <span>{copy.room}</span> }} /><span className="brd-section-icon"><States value={state.scene === 'room' ? 'room' : 'board'} states={{ board: <Motorbike size={16} strokeWidth={2} />, room: <MessageCircle size={16} strokeWidth={2} /> }} /></span></div>
          <div className="brd-scenes" data-scene={state.scene}>
            <div className="brd-pane" data-on={state.scene === 'board' || undefined}>
              <div className="brd-ride" data-target="ride" data-cursor="pointer"><Row title={<span className="brd-place"><MapPin size={16} strokeWidth={2} />{copy.city}</span>} meta={copy.date} subtitle={copy.description} pad={12}><ChevronRight size={16} strokeWidth={2} /></Row></div>
              <div className="brd-other-ride"><Row title={copy.otherCity} subtitle={copy.otherDescription} meta={copy.date} pad={12} /></div>
              <div className="brd-privacy"><LockKeyhole size={16} strokeWidth={2} /><span>{copy.private}</span></div>
            </div>
            <div className="brd-pane" data-on={['preview', 'pending', 'approved'].includes(state.scene) || undefined}>
              <div className="brd-preview-card"><div className="brd-city"><MapPin size={16} strokeWidth={2} /><strong>{copy.city}</strong><span><CalendarDays size={16} strokeWidth={2} />{copy.date}</span></div><div className="brd-description">{copy.description}</div><div className="brd-approval-state"><States value={state.scene === 'approved' ? 'approved' : state.scene === 'pending' ? 'pending' : 'private'} align="start" states={{ private: <span><LockKeyhole size={16} strokeWidth={2} />{copy.private}</span>, pending: <span><Clock3 size={16} strokeWidth={2} />{copy.waiting}</span>, approved: <span className="brd-approved"><Check size={16} strokeWidth={2} />{copy.approved}</span> }} /></div></div>
              <div className="brd-cta" data-target={state.scene === 'approved' ? 'room' : 'request'} data-cursor="pointer" data-pending={state.scene === 'pending' || undefined}><States value={state.scene === 'approved' ? 'approved' : state.scene === 'pending' ? 'pending' : 'request'} states={{ request: <span>{copy.request}<ArrowRight size={16} strokeWidth={2} /></span>, pending: <span>{copy.pending}<Check size={16} strokeWidth={2} /></span>, approved: <span>{copy.open}<ArrowRight size={16} strokeWidth={2} /></span> }} /></div>
            </div>
            <div className="brd-pane brd-room" data-on={state.scene === 'room' || undefined}>
              <div className="brd-room-context"><span><MapPin size={16} strokeWidth={2} />{copy.city} · {copy.date}</span><Badge tone="success" style={{ fontSize: 12 }}>{copy.approvedGroup}</Badge></div>
              <div className="brd-plan"><Row title={copy.meeting} subtitle={copy.examplePoint} lead={<MapPin size={16} strokeWidth={2} />} pad={12} /></div>
              <div className="brd-chat"><div className="brd-chat-label"><MessageCircle size={16} strokeWidth={2} />{copy.chat}</div><div className="brd-organizer">{copy.organizer}</div><Thread app="chat"><Bubble from="them">{copy.message}</Bubble></Thread></div>
            </div>
          </div>
          <div className="brd-filler" aria-hidden="true"><Bar w="78%" h={6} tone={1} /><Bar w="53%" h={6} tone={1} /></div>
        </div></Window>}
      </Stage>
}
