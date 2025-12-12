import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {createRoot} from 'react-dom/client';
import * as Moment from 'moment';
import {App} from './app';

const container = document.getElementById('app');
const root = createRoot(container!);
root.render(<App />);

const mdl = module as any;
if (mdl.hot) {
    mdl.hot.accept('./app.tsx', () => {
        const UpdatedApp = require('./app.tsx').App;
        root.render(<UpdatedApp />);
    });
}

(window as any).React = React;
(window as any).ReactDOM = ReactDOM;
(window as any).Moment = Moment;
