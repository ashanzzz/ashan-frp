import { BrowserRouter } from 'react-router-dom'
import { GlobalStateProvider } from './hooks/useGlobalState'
import MainLayout from './layouts/MainLayout'
import './App.css'

export default function App() {
  return (
    <BrowserRouter>
      <GlobalStateProvider>
        <MainLayout />
      </GlobalStateProvider>
    </BrowserRouter>
  )
}