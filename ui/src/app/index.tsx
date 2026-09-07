import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {createRoot} from 'react-dom/client';
import * as Moment from 'moment';
import {App} from './app';

const container = document.getElementById('app');
const root = createRoot(container!);
root.render(<App />);

(window as any).React = React;
(window as any).ReactDOM = ReactDOM;
(window as any).Moment = Moment;
(window as any).ReactJSXRuntime = require('react/jsx-runtime');
