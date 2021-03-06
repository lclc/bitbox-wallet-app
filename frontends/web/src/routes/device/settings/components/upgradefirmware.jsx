/**
 * Copyright 2018 Shift Devices AG
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Component } from 'preact';
import { translate } from 'react-i18next';
import { Button } from '../../../../components/forms';
import Dialog from '../../../../components/dialog/dialog';
import WaitDialog from '../../../../components/wait-dialog/wait-dialog';
import { apiGet, apiPost } from '../../../../utils/request';
import componentStyle from '../../../../components/style.css';

@translate()
export default class UpgradeFirmware extends Component {
    state = {
        unlocked: false,
        newVersion: '',
        isConfirming: false,
        activeDialog: false,
    }

    upgradeFirmware = () => {
        this.setState({
            activeDialog: false,
            isConfirming: true,
        });
        apiPost('devices/' + this.props.deviceID + '/unlock-bootloader').then((success) => {
            this.setState({
                unlocked: success,
                isConfirming: success,
            });
        }).catch(e => {
            this.setState({
                isConfirming: false,
            });
        });
    };

    componentDidMount() {
        apiGet('devices/' + this.props.deviceID + '/bundled-firmware-version').then(version => {
            this.setState({ newVersion: version.replace('v', '') });
        });
    }

    render({
        t,
        currentVersion,
        disabled,
    }, {
        unlocked,
        newVersion,
        isConfirming,
        activeDialog,
    }) {
        return (
            <div>
                <Button
                    primary
                    onClick={() => this.setState({ activeDialog: true })}
                    disabled={disabled}>
                    {t('upgradeFirmware.button')}
                    {
                        currentVersion !== null && newVersion !== currentVersion && (
                            <div class={componentStyle.badge}>1</div>
                        )
                    }
                </Button>
                {
                    activeDialog && (
                        <Dialog title={t('upgradeFirmware.title')}>
                            <p>{t('upgradeFirmware.description', {
                                currentVersion, newVersion
                            })}</p>
                            <div class={['flex', 'flex-row', 'flex-end', 'buttons'].join(' ')}>
                                <Button secondary onClick={() => this.setState({ activeDialog: false })}>
                                    {t('button.back')}
                                </Button>
                                <Button primary onClick={this.upgradeFirmware}>
                                    {t('button.upgrade')}
                                </Button>
                            </div>
                        </Dialog>
                    )
                }
                {
                    isConfirming && (
                        <WaitDialog title={t('upgradeFirmware.title')} includeDefault={!unlocked}>
                            {
                                unlocked ? (
                                    <div>
                                        <p>{t('upgradeFirmware.unlocked')}</p>
                                        <ul style="line-height: 1.5;">
                                            <li>{t('upgradeFirmware.unlocked1')}</li>
                                            <li>{t('upgradeFirmware.unlocked2')}</li>
                                        </ul>
                                    </div>
                                ) : (
                                    <p>{t('upgradeFirmware.locked', {
                                        currentVersion, newVersion
                                    })}</p>
                                )
                            }
                        </WaitDialog>
                    )
                }
            </div>
        );
    }
}
