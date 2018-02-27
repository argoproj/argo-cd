import * as React from 'react';
import * as ReactDOM from 'react-dom';
import { App, store } from './app';

ReactDOM.render(<App store={store}/>, document.getElementById('app'));

const mdl = module as any;
if (mdl.hot) {
    mdl.hot.accept('./app.tsx', () => {
        const UpdatedApp = require('./app.tsx').App;
        ReactDOM.render(<UpdatedApp/>, document.getElementById('app'));
    });
}
