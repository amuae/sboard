/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare module 'bootstrap' {
  export class Modal {
    constructor(element: Element | null, options?: object)
    show(): void
    hide(): void
    toggle(): void
    dispose(): void
    static getInstance(element: Element): Modal | null
    static getOrCreateInstance(element: Element): Modal
  }
}
