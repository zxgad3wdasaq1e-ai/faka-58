import { build } from 'vite'

// Keep the same build contract on Windows and POSIX shells.
process.env.VITE_FULLSTACK = '1'
await build()
