/**
 * Copyright 2020 Google LLC
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

/**
 * track if the user is logged in
 */
import {Injectable} from '@angular/core';
import {User} from './user.model';

@Injectable()
export class AuthService {
  private user: User;
  loggedInStatus = false;

  constructor() {
    this.user = new User();
  }

  setLoggedIn(value: boolean) {
    this.loggedInStatus = value;
  }

  setUsername(username: string) {
    this.user.username = username;
  }

  get getUsername() {
    return this.user.username;
  }

  get isLoggedIn() {
    return this.loggedInStatus;
  }

  authUser() {
    return this.user;
  }
}
