import './insert-nonce';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {App} from './app';

// Import this directly, because we use the CSS-less version of Tippy (wee webpack.config.js config.resolve.js).
import './tippy.js-4.3.5-index.css';

ReactDOM.render(<App />, document.getElementById('app'));

const mdl = module as any;
if (mdl.hot) {
    mdl.hot.accept('./app.tsx', () => {
        const UpdatedApp = require('./app.tsx').App;
        ReactDOM.render(<UpdatedApp />, document.getElementById('app'));
    });
}

(window as any).React = React;
