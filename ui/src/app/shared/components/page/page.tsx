import {DataLoader, Toolbar, Utils} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import Helmet from 'react-helmet';
import {BehaviorSubject, Observable} from 'rxjs';
import {map} from 'rxjs/operators';
import {Link} from 'react-router-dom';

import {Context, ContextApis} from '../../context';
import {services} from '../../services';
import requests from '../../services/requests';

const mostRecentLoggedIn = new BehaviorSubject<boolean>(false);

import './page.scss';

function isLoggedIn(): Observable<boolean> {
    services.users.get().then(info => mostRecentLoggedIn.next(info.loggedIn));
    return mostRecentLoggedIn;
}

export const AddAuthToToolbar = (init: Toolbar | Observable<Toolbar>, ctx: ContextApis): Observable<Toolbar> => {
    return Utils.toObservable(init).pipe(
        map(toolbar => {
            toolbar = toolbar || {};
            toolbar.tools = [
                toolbar.tools,
                <DataLoader key='loginPanel' load={() => isLoggedIn()}>
                    {loggedIn => (
                        <div style={{marginLeft: '5px', display: 'inline', flexShrink: 0}}>
                            {loggedIn ? (
                                <button className='page__button' key='logout' onClick={() => (window.location.href = requests.toAbsURL('/auth/logout'))}>
                                    <i className='fa fa-right-to-bracket' /> Log Out
                                </button>
                            ) : (
                                <button className='page__button' key='login' onClick={() => ctx.navigation.goto(`/login?return_url=${encodeURIComponent(location.href)}`)}>
                                    <i className='fa fa-right-to-bracket' /> Log In
                                </button>
                            )}
                        </div>
                    )}
                </DataLoader>
            ];
            return toolbar;
        })
    );
};

interface PageProps extends React.Props<any> {
    title: string;
    hideAuth?: boolean;
    toolbar?: Toolbar | Observable<Toolbar>;
    topBarTitle?: string;
    useTitleOnly?: boolean;
}

export const Page = (props: PageProps) => {
    const ctx = React.useContext(Context);
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {pref => (
                <div className={`${props.hideAuth ? 'page-wrapper' : ''} ${!!pref.hideSidebar ? 'sb-page-wrapper__sidebar-collapsed' : 'sb-page-wrapper'}`}>
                    <ArgoPage
                        title={props.title}
                        children={props.children}
                        topBarTitle={props.topBarTitle}
                        useTitleOnly={props.useTitleOnly}
                        toolbar={!props.hideAuth ? AddAuthToToolbar(props.toolbar, ctx) : props.toolbar}
                    />
                </div>
            )}
        </DataLoader>
    );
};

interface ArgoPageProps extends React.Props<any> {
    title: string;
    toolbar?: Toolbar | Observable<Toolbar>;
    topBarTitle?: string;
    useTitleOnly?: boolean;
}

export interface PageContextProps {
    title: string;
}

export const PageContext = React.createContext<PageContextProps>({title: 'Argo'});

export const ArgoPage = (props: ArgoPageProps) => {
    const toolbarObservable = props.toolbar && Utils.toObservable(props.toolbar);

    return (
        <div className={classNames('page', {'page--has-toolbar': !!props.toolbar})}>
            <React.Fragment>
                {toolbarObservable && (
                    <DataLoader input={new Date()} load={() => toolbarObservable}>
                        {(toolbar: Toolbar) => (
                            <React.Fragment>
                                <PageContext.Consumer>
                                    {ctx => {
                                        let titleParts = [ctx.title];
                                        if (!props.useTitleOnly && toolbar && toolbar.breadcrumbs && toolbar.breadcrumbs.length > 0) {
                                            titleParts = [
                                                toolbar.breadcrumbs
                                                    .map(item => item.title)
                                                    .reverse()
                                                    .join(' / ')
                                            ].concat(titleParts);
                                        } else if (props.title) {
                                            titleParts = [props.title].concat(titleParts);
                                        }
                                        return (
                                            <Helmet>
                                                <title>{titleParts.join(' - ')}</title>
                                            </Helmet>
                                        );
                                    }}
                                </PageContext.Consumer>
                                <div className='page__top-bar'>
                                    <TopBar title={props.topBarTitle ? props.topBarTitle : props.title} toolbar={toolbar} />
                                </div>
                            </React.Fragment>
                        )}
                    </DataLoader>
                )}
                <div className='page__content-wrapper'>{props.children}</div>
            </React.Fragment>
        </div>
    );
};

export interface TopBarProps extends React.Props<any> {
    title: string;
    toolbar?: Toolbar;
}
export interface ActionMenu {
    className?: string;
    items: {
        action: () => any;
        title: string | React.ReactElement;
        iconClassName?: string;
        qeId?: string;
        disabled?: boolean;
    }[];
}

const renderActionMenu = (actionMenu: ActionMenu) => (
    <div>
        {actionMenu.items.map((item, i) => (
            <button disabled={!!item.disabled} qe-id={item.qeId} className='page__button' onClick={() => item.action()} style={{marginRight: 4}} key={i}>
                {item.iconClassName && <i className={item.iconClassName} style={{marginLeft: '-5px', marginRight: '5px'}} />}
                {item.title}
            </button>
        ))}
    </div>
);

export const RenderToolbar = (toolbar: Toolbar) => (
    <div className='top-bar row toolbar' key='tool-bar'>
        <div className='top-bar__left-side'>{toolbar.actionMenu && renderActionMenu(toolbar.actionMenu)}</div>
        <div style={{marginLeft: 'auto', marginRight: '1em', display: 'flex', alignItems: 'center', justifyContent: 'flex-end', flexGrow: 1}}>{toolbar.tools}</div>
    </div>
);

const renderBreadcrumbs = (breadcrumbs: {title: string | React.ReactNode; path?: string}[]) => (
    <div className='top-bar__breadcrumbs'>
        {(breadcrumbs || []).map((breadcrumb, i) => {
            const nodes = [];
            if (i === breadcrumbs.length - 1) {
                nodes.push(
                    <span key={i} className='top-bar__breadcrumbs-last-item'>
                        {breadcrumb.title}
                    </span>
                );
            } else {
                nodes.push(
                    <Link key={i} to={breadcrumb.path}>
                        {' '}
                        {breadcrumb.title}{' '}
                    </Link>
                );
            }
            if (i < breadcrumbs.length - 1) {
                nodes.push(<span key={`${i}_sep`} className='top-bar__sep' />);
            }
            return nodes;
        })}
    </div>
);

export const TopBar = (props: TopBarProps) => (
    <div>
        <div className='top-bar' key='top-bar'>
            <div className='row'>
                <div className='columns top-bar__left-side'>{props.toolbar && props.toolbar.breadcrumbs && renderBreadcrumbs(props.toolbar.breadcrumbs)}</div>
                <div className='top-bar__title text-truncate top-bar__right-side'>{props.title}</div>
            </div>
        </div>
        {props.toolbar && RenderToolbar(props.toolbar)}
    </div>
);
