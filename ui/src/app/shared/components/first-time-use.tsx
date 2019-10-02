import * as React from 'react';
// @ts-ignore
import Modal from 'react-modal';

const localStorageKey = 'first-time-usage';

class State {
    public readKeys: string[];
    public hide: boolean;
}

class Props {
    public key: string;
    public children: React.ReactNode;
}

// https://www.robinwieruch.de/local-storage-react
export class FTU extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = JSON.parse(window.localStorage.getItem(localStorageKey) || '{}') as State;
    }

    public render() {
        return (
            <Modal
                style={{overlay: {zIndex: 999}, content: {bottom: 'auto'}}}
                isOpen={this.isOpen()}
                appElement={document.getElementById('app')}>
                {this.props.children}
                <p style={{textAlign: 'right'}}>
                    <button className='argo-button argo-button--base-o' onClick={() => this.close()}><i
                        className='fa fa-times'/> Close
                    </button>
                    &nbsp;
                    <button className='argo-button argo-button--base-o' onClick={() => this.closeForever()}><i
                        className='fa fa-times'/> Don't ask again
                    </button>
                </p>
            </Modal>
        );
    }

    private close() {
        this.setState((prevState) => {
            const s = {readKeys: prevState.readKeys || []} as State;
            s.hide = true;
            return s;
        });
    }

    private closeForever() {
        this.close();
        window.localStorage.setItem(localStorageKey, JSON.stringify(this.state));
    }

    private isOpen() {
        const unread = !this.state || !this.state.readKeys || this.state.readKeys.find((i) => i === this.props.key) === undefined;
        return !this.state.hide && unread;
    }
}
