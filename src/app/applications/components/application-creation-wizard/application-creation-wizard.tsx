import { MockupList, NotificationType } from 'argo-ui';
import * as path from 'path';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { BehaviorSubject } from 'rxjs';

import { DataLoader, ErrorNotification } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

import { AppParams, AppsList, EnvironmentsList, NewAppParams, RepositoryList } from './steps';

export { NewAppParams } from './steps';

enum Step { SelectRepo = 0, SelectApp = 1, SelectEnvironments = 2, SetParams = 3 }
interface StepInfo { title: string | React.ReactNode; canNext(): boolean; next(): any; render(): React.ReactNode; canPrev(): boolean; prev(): any; }

interface State {
    envs: { [key: string]: models.KsonnetEnvironment; };
    selectedRepo: string;
    revision: string;
    editingRevision: string;
    selectedApp: models.AppInfo;
    selectedAppDetails: models.AppDetails;
    selectedEnv: string;
    appParams: NewAppParams;
    appParamsValid: boolean;
    step: Step;
    loading: boolean;
    projects: models.Project[];
}

export interface WizardProps { onStateChanged: (state: WizardStepState) => any; createApp: (params: NewAppParams) => any; }

export interface WizardStepState { nextTitle: string; next?: () => any; prev?: () => any; }

require('./application-creation-wizard.scss');

export class ApplicationCreationWizardContainer extends React.Component<WizardProps, State> {
    public static contextTypes = {
        apis: PropTypes.object,
    };

    private submitAppParamsForm = new BehaviorSubject<any>(null);
    private appsLoader: DataLoader;

    constructor(props: WizardProps) {
        super(props);
        this.state = {
            envs: {},
            selectedAppDetails: null,
            selectedRepo: null,
            revision: 'HEAD',
            editingRevision: null,
            selectedEnv: null,
            selectedApp: null,
            appParamsValid: false,
            step: Step.SelectRepo,
            loading: false,
            appParams: null,
            projects: [],
        };
        this.notifyStepStateChanged();
    }

    public async componentDidMount() {
        const projects = (await services.projects.list()).sort((a, b) => {
            if (a.metadata.name === 'default') {
                return -1;
            }
            if (b.metadata.name  === 'default') {
                return 1;
            }
            return a.metadata.name.localeCompare(b.metadata.name );
        });
        this.setState({projects});
    }

    public render() {
        const currentStep = this.getCurrentStep();

        return (
            <div>
                <h4>{currentStep.title}</h4>
                {currentStep.render()}
            </div>
        );
    }

    private getCurrentStep(): StepInfo {
        switch (this.state.step) {
            case Step.SelectApp:
                return {
                    title: (
                        <div className='application-creation-wizard__title'>
                            Select application
                            <button className='argo-button argo-button--base application-creation-wizard__title-btn'
                                onClick={() => !this.state.loading && this.updateState({ selectedApp: null, selectedAppDetails: null, appParams: {
                                applicationName: '',
                                revision: this.state.revision,
                                repoURL: this.state.selectedRepo,
                                environment: '',
                                clusterURL: '',
                                namespace: '',
                                path: '',
                                project: this.state.projects[0].metadata.name,
                            }, step: Step.SetParams })}>Create app from directory</button>
                            <div className='application-creation-wizard__sub-title'>
                                Browsing repository <a target='_blank' href={this.state.selectedRepo}>{this.state.selectedRepo}</a>, revision:
                                    {this.state.editingRevision === null && <React.Fragment>
                                        <span className='application-creation-wizard__rev'>{this.state.revision || 'HEAD'}</span>
                                        <i className='fa fa-pencil' onClick={() => this.setState({ editingRevision: this.state.revision })}/>
                                    </React.Fragment>}
                                    {this.state.editingRevision !== null && <React.Fragment>
                                        <input className='application-creation-wizard__rev'
                                            value={this.state.editingRevision || 'HEAD'} onChange={(e) => this.setState({ editingRevision: e.target.value })}/>
                                        <i className='fa fa-check' onClick={() => {
                                            this.setState({editingRevision: null, revision: this.state.editingRevision});
                                            this.appsLoader.reload();
                                        }}/>
                                        <i className='fa fa-times' onClick={() => this.setState({editingRevision: null})}/>
                                    </React.Fragment>}
                            </div>
                        </div>
                    ),
                    canNext: () => !!this.state.selectedApp,
                    next: async () => {
                        try {
                            this.updateState({ loading: true });
                            const selectedAppDetails = await services.repos.appDetails(this.state.selectedRepo, this.state.selectedApp.path, this.state.revision);

                            if (selectedAppDetails.ksonnet) {
                                this.updateState({ selectedAppDetails, envs: selectedAppDetails.ksonnet.environments || {}, step: Step.SelectEnvironments});
                            } else if (selectedAppDetails.helm) {
                                this.updateState({ selectedAppDetails, appParams: {
                                    applicationName: selectedAppDetails.helm.name,
                                    revision: this.state.revision,
                                    repoURL: this.state.selectedRepo,
                                    environment: '',
                                    clusterURL: '',
                                    namespace: '',
                                    path: path.dirname(selectedAppDetails.helm.path),
                                    project: this.state.projects[0].metadata.name,
                                }, step: Step.SetParams });
                            } else if (selectedAppDetails.kustomize) {
                                this.updateState({ selectedAppDetails, appParams: {
                                    applicationName: '',
                                    revision: this.state.revision,
                                    repoURL: this.state.selectedRepo,
                                    environment: '',
                                    clusterURL: '',
                                    namespace: '',
                                    path: path.dirname(selectedAppDetails.kustomize.path),
                                    project: this.state.projects[0].metadata.name,
                                }, step: Step.SetParams });
                            }
                        } catch (e) {
                            this.appContext.apis.notifications.show({type: NotificationType.Error, content: <ErrorNotification title='Unable to load app details' e={e} />});
                        } finally {
                            this.updateState({ loading: false });
                        }
                    },
                    canPrev: () => true,
                    prev: () => this.updateState({ step: Step.SelectRepo, revision: 'HEAD' }),
                render: () => (
                    <DataLoader ref={(loader) => this.appsLoader = loader} key='apps'
                            load={() => services.repos.apps(this.state.selectedRepo, this.state.revision)}
                            loadingRenderer={() => <MockupList height={50} marginTop={10}/>}>
                        {(apps: models.AppInfo[]) => (
                            <AppsList apps={apps} selectedApp={this.state.selectedApp} onAppSelected={(selectedApp) => this.updateState({ selectedApp })}/>
                        )}
                    </DataLoader>
                ),
                };
            case Step.SelectEnvironments:
                return {
                    title: 'Select environment',
                    canNext: () => !!this.state.selectedEnv,
                    next: async () => {
                        const selectedEnv = this.state.envs[this.state.selectedEnv];
                        this.updateState({
                            appParams: {
                                applicationName: `${this.state.selectedAppDetails.ksonnet.name}-${this.state.selectedEnv}`,
                                revision: this.state.revision,
                                repoURL: this.state.selectedRepo,
                                environment: this.state.selectedEnv,
                                clusterURL: selectedEnv.destination.server,
                                namespace: selectedEnv.destination.namespace,
                                path: path.dirname(this.state.selectedAppDetails.ksonnet.path),
                                project: this.state.projects[0].metadata.name,
                            }, step: Step.SetParams,
                        });
                    },
                    canPrev: () => true,
                    prev: () => this.updateState({ step: Step.SelectApp }),
                    render: () => (
                        <EnvironmentsList envs={this.state.envs} selectedEnv={this.state.selectedEnv} onEnvsSelectionChanged={(env) => this.updateState({ selectedEnv: env })}/>
                    ),
                };
            case Step.SetParams:
                return {
                    title: 'Review application parameters',
                    canNext: () => this.state.appParamsValid,
                    next: async () => this.submitAppParamsForm.next({}),
                    canPrev: () => true,
                    prev: async () => {
                        if (this.state.selectedAppDetails && this.state.selectedAppDetails.ksonnet) {
                            this.updateState({ step: Step.SelectEnvironments });
                        } else {
                            this.updateState({ step: Step.SelectApp });
                        }
                    },
                    render: () => (
                        <AppParams
                            needKsonnetParams={!!(this.state.selectedAppDetails && this.state.selectedAppDetails.ksonnet)}
                            needHelmParams={!!(this.state.selectedAppDetails && this.state.selectedAppDetails.helm)}
                            needKustomizeParams={!!(this.state.selectedAppDetails && this.state.selectedAppDetails.kustomize)}
                            environments={this.state.selectedAppDetails && this.state.selectedAppDetails.ksonnet
                                && Object.keys(this.state.selectedAppDetails.ksonnet.environments) || []}
                            valueFiles={this.state.selectedAppDetails && this.state.selectedAppDetails.helm && this.state.selectedAppDetails.helm.valueFiles || []}
                            projects={this.state.projects}
                            appParams={this.state.appParams}
                            submitForm={this.submitAppParamsForm}
                            onSubmit={this.props.createApp}
                            onValidationChanged={(isValid) => this.updateState({ appParamsValid: isValid })} />
                    ),
                };
            case Step.SelectRepo:
            default:
                const isRepoValid = this.state.selectedRepo && /((git|ssh|http(s)?)|(git@[\w.]+))(:(\/\/)?)([\w.@:/\-~]+)(\/)?/.test(this.state.selectedRepo);
                return {
                    title: 'Select repository',
                    canNext: () => !!this.state.selectedRepo && isRepoValid,
                    next: async () => this.updateState({ step: Step.SelectApp, selectedRepo: this.state.selectedRepo.trim() }),
                    canPrev: () => false,
                    prev: null,
                    render: () => (
                        <DataLoader key='repos' load={() => services.repos.list()} loadingRenderer={() => <MockupList height={50} marginTop={10}/>}>
                        {(repos) => <RepositoryList
                            selectedRepo={this.state.selectedRepo}
                            invalidRepoURL={this.state.selectedRepo && !isRepoValid}
                            repos={repos}
                            onSelectedRepo={(repo) => this.updateState({ selectedRepo: repo })}/>}
                        </DataLoader>
                    ),
                };
        }
    }

    private updateState<K extends keyof State>(update: (Pick<State, K> | State)) {
        this.setState(update, () => {
            this.notifyStepStateChanged();
        });
    }

    private notifyStepStateChanged() {
        const currentStep = this.getCurrentStep();
        this.props.onStateChanged({
            next: currentStep.canNext() && !this.state.loading && currentStep.next,
            prev: currentStep.canPrev()  && !this.state.loading && currentStep.prev,
            nextTitle: this.state.step === Step.SetParams ? 'Create' : 'Next',
        });
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
