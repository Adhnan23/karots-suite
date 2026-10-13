import { useReducer } from 'react'
import { errMsg } from '@/features/till/api'
import { askReducer, type AskState } from './ask'
import type { AskReply } from './types'

/** Runs one AI action through askReducer: ask() sends without a reply, read(text) sends the pasted one. */
export function useAsk<T>() {
  const [state, dispatch] = useReducer(askReducer<T>, { k: 'idle' } as AskState<T>)
  async function ask(call: () => Promise<AskReply<T>>) {
    dispatch({ t: 'ask' })
    try {
      dispatch({ t: 'answer', a: await call() })
    } catch (e) {
      dispatch({ t: 'fail', error: errMsg(e) })
      throw e
    }
  }
  async function read(reply: string, call: (reply: string) => Promise<AskReply<T>>) {
    if (!reply.trim()) return
    dispatch({ t: 'type', reply })
    dispatch({ t: 'read' })
    try {
      dispatch({ t: 'answer', a: await call(reply) })
    } catch (e) {
      dispatch({ t: 'fail', error: errMsg(e) })
    }
  }
  return { state, dispatch, ask, read }
}
