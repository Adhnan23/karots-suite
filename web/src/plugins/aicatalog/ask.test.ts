import { describe, expect, it } from 'vitest'
import { type AskState, askReducer, looksLikeAnswer } from './ask'

type R = { name: string }
const run = (acts: Parameters<typeof askReducer<R>>[1][], s: AskState<R> = { k: 'idle' }) => acts.reduce(askReducer<R>, s)

describe('askReducer', () => {
  it('API answer goes straight to done', () => {
    expect(run([{ t: 'ask' }, { t: 'answer', a: { result: { name: 'Coke' } } }])).toEqual({ k: 'done', result: { name: 'Coke' } })
  })
  it('paste mode: prompt, paste, read, done', () => {
    const s = run([{ t: 'ask' }, { t: 'answer', a: { prompt: 'P' } }, { t: 'type', reply: '{"x":1}' }, { t: 'read' }])
    expect(s).toEqual({ k: 'reading', prompt: 'P', reply: '{"x":1}' })
    expect(askReducer<R>(s, { t: 'answer', a: { result: { name: 'A' } } }).k).toBe('done')
  })
  it('a bad paste keeps the text and says why', () => {
    const s = run([{ t: 'answer', a: { prompt: 'P' } }, { t: 'type', reply: 'sorry no idea' }, { t: 'read' }, { t: 'answer', a: { prompt: 'P', error: 'No JSON' } }])
    expect(s).toEqual({ k: 'awaiting', prompt: 'P', reply: 'sorry no idea', error: 'No JSON' })
    expect(askReducer<R>(s, { t: 'type', reply: 'again' })).toEqual({ k: 'awaiting', prompt: 'P', reply: 'again', error: undefined })
  })
  it('API failure offers the prompt with the error', () => {
    expect(run([{ t: 'ask' }, { t: 'answer', a: { prompt: 'P', error: 'overloaded' } }])).toEqual({ k: 'awaiting', prompt: 'P', reply: '', error: 'overloaded' })
  })
  it('a network failure while reading keeps the reply', () => {
    const s = run([{ t: 'answer', a: { prompt: 'P' } }, { t: 'type', reply: '{}' }, { t: 'read' }, { t: 'fail', error: 'offline' }])
    expect(s).toEqual({ k: 'awaiting', prompt: 'P', reply: '{}', error: 'offline' })
  })
  it('an empty reply is not read', () => {
    expect(run([{ t: 'answer', a: { prompt: 'P' } }, { t: 'read' }]).k).toBe('awaiting')
  })
})

describe('looksLikeAnswer', () => {
  it('finds JSON inside prose and fences', () => {
    expect(looksLikeAnswer('Here:\n```json\n{"confident":true}\n```')).toBe(true)
    expect(looksLikeAnswer('just some copied text')).toBe(false)
    expect(looksLikeAnswer('{not json}')).toBe(false)
  })
})
