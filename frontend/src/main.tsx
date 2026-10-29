import React from 'react';
import ReactDOM from 'react-dom/client';

import App from './App';
import './styles/base.css';
import './styles/app.css';
import './styles/reader.css';
import './styles/skins.css';

async function bootstrap() {
  if (import.meta.env.DEV && !(window as unknown as { go?: unknown }).go) {
    const { installDevMock } = await import('./devMock');
    installDevMock();
  }
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  );
}

void bootstrap();
