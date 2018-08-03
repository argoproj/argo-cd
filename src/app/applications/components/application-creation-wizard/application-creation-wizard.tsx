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
    selectedApp: models.AppInfo;
    selectedAppDetails: models.AppDetails;
    selectedEnv: string;
    appParams: NewAppParams;
    appParamsValid: boolean;
    step: Step;
    loading: boolean;
    projects: string[];
}

export interface WizardProps { onStateChanged: (state: WizardStepState) => any; createApp: (params: NewAppParams) => any; }

export interface WizardStepState { nextTitle: string; next?: () => any; prev?: () => any; }

export class ApplicationCreationWizardContainer extends React.Component<WizardProps, State> {
    public static contextTypes = {
        apis: PropTypes.object,
    };

    private submitAppParamsForm = new BehaviorSubject<any>(null);

    constructor(props: WizardProps) {
        super(props);
        this.state = {
            envs: {},
            selectedAppDetails: null,
            selectedRepo: null,
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
        const projects = (await services.projects.list()).map((proj) => proj.metadata.name).sort((a, b) => {
            if (a === 'default') {
                return -1;
            }
            if (b === 'default') {
                return 1;
            }
            return a.localeCompare(b);
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
                        <div>Select application or <a onClick={() => !this.state.loading && this.updateState({ selectedApp: null, selectedAppDetails: null, appParams: {
                            applicationName: '',
                            repoURL: this.state.selectedRepo,
                            environment: '',
                            clusterURL: '',
                            namespace: '',
                            path: '',
                            project: this.state.projects[0],
                        }, step: Step.SetParams })}>specify</a> drop-in YAML directory</div>
                    ),
                    canNext: () => !!this.state.selectedApp,
                    next: async () => {
                        try {
                            this.updateState({ loading: true });
                            const selectedAppDetails = await services.reposService.appDetails(this.state.selectedRepo, this.state.selectedApp.path);

                            if (selectedAppDetails.ksonnet) {
                                this.updateState({ selectedAppDetails, envs: selectedAppDetails.ksonnet.environments || {}, step: Step.SelectEnvironments});
                            } else {
                                this.updateState({ selectedAppDetails, appParams: {
                                    applicationName: selectedAppDetails.helm.name,
                                    repoURL: this.state.selectedRepo,
                                    environment: '',
                                    clusterURL: '',
                                    namespace: '',
                                    path: path.dirname(selectedAppDetails.helm.path),
                                    project: this.state.projects[0],
                                }, step: Step.SetParams });
                            }
                        } catch (e) {
                            this.appContext.apis.notifications.show({type: NotificationType.Error, content: <ErrorNotification title='Unable to load app details' e={e} />});
                        } finally {
                            this.updateState({ loading: false });
                        }
                    },
                    canPrev: () => true,
                    prev: () => this.updateState({ step: Step.SelectRepo }),
                render: () => (
                    <DataLoader key='apps' load={() => services.reposService.apps(this.state.selectedRepo)} loadingRenderer={() => <MockupList height={50} marginTop={10}/>}>
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
                                repoURL: this.state.selectedRepo,
                                environment: this.state.selectedEnv,
                                clusterURL: selectedEnv.destination.server,
                                namespace: selectedEnv.destination.namespace,
                                path: path.dirname(this.state.selectedAppDetails.ksonnet.path),
                                project: this.state.projects[0],
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
                            needEnvironment={!!(this.state.selectedAppDetails && this.state.selectedAppDetails.ksonnet)}
                            environments={this.state.selectedAppDetails &&
                                this.state.selectedAppDetails.ksonnet && Object.keys(this.state.selectedAppDetails.ksonnet.environments) || []}
                            projects={this.state.projects}
                            appParams={this.state.appParams}
                            submitForm={this.submitAppParamsForm}
                            onSubmit={this.props.createApp}
                            onValidationChanged={(isValid) => this.updateState({ appParamsValid: isValid })} />
                    ),
                };
            case Step.SelectRepo:
            default:
                return {
                    title: 'Select repository',
                    canNext: () => !!this.state.selectedRepo,
                    next: async () => this.updateState({ step: Step.SelectApp }),
                    canPrev: () => false,
                    prev: null,
                    render: () => (
                        <DataLoader key='repos' load={() => services.reposService.list()} loadingRenderer={() => <MockupList height={50} marginTop={10}/>}>
                        {(repos) => <RepositoryList
                            selectedRepo={this.state.selectedRepo}
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
