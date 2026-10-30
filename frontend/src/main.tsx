import React from 'react';
import ReactDOM from 'react-dom/client';

import App from './App';
import { ErrorBoundary } from './components/ErrorBoundary';
import './styles/base.css';
import './styles/app.css';
import './styles/reader.css';
import './styles/skins.css';
import './styles/chat.css';
import './styles/responsive.css';

async function bootstrap() {
  if (import.meta.env.DEV && !(window as unknown as { go?: unknown }).go) {
    const { installDevMock } = await import('./devMock');
    installDevMock();
  }
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </React.StrictMode>,
  );
}

void bootstrap();
