import {WebDriver} from 'selenium-webdriver';

export abstract class Base {
    protected driver: WebDriver;

    public constructor(driver: WebDriver) {
        this.driver = driver;
    }
}
