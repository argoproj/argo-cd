import {By, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import UiTestUtilities from '../UiTestUtilities';

// Popup Confirmation dialog
// Uncomment to use
// const POPUP_OK_BUTTON_CSS: By = By.css('#app .popup-container .qe-argo-popup-ok-button');
// const POPUP_OK_BUTTON: By = By.xpath('.//button[@qe-id="argo-popup-ok-button"]');
// const POPUP_CANCEL_BUTTON: By = By.xpath('.//button[@qe-id="argo-popup-cancel-button"]');

// Popup Prompt dialog
const DELETE_APP_POPUP_FIELD_CONFIRMATION: By = By.xpath('.//input[@qeid="name-field-delete-confirmation"]');
const DELETE_APP_PROMPT_POPUP_OK_BUTTON: By = By.xpath('.//button[@qe-id="prompt-popup-ok-button"]');
const DELETE_APP_PROMPT_POPUP_CANCEL_BUTTON: By = By.xpath('.//button[@qe-id="prompt-popup-cancel-button"]');

export class PopupManager extends Base {
    public constructor(driver: WebDriver) {
        super(driver);
    }

    public async setPromptFieldName(appName: string): Promise<void> {
        const confirmationField = await UiTestUtilities.findUiElement(this.driver, DELETE_APP_POPUP_FIELD_CONFIRMATION);
        await confirmationField.sendKeys(appName);
    }

    public async clickPromptOk(): Promise<void> {
        const okButton = await UiTestUtilities.findUiElement(this.driver, DELETE_APP_PROMPT_POPUP_OK_BUTTON);
        await okButton.click();
    }

    public async clickPromptCancel(): Promise<void> {
        const cancelButton = await UiTestUtilities.findUiElement(this.driver, DELETE_APP_PROMPT_POPUP_CANCEL_BUTTON);
        await cancelButton.click();
    }
}
