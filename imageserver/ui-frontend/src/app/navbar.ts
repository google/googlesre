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

import {Component, OnInit} from '@angular/core';
import {Router} from '@angular/router';
import {Observable} from 'rxjs';

import {AuthService} from './authservice';
import {User} from './user.model';

@Component({
  selector: 'app-navbar',
  templateUrl: './navbar.ng.html',
  styleUrls: ['./navbar.scss']
})
export class Navbar {
  title: string;
  user: User;

  constructor(private auth: AuthService, private router: Router) {
    this.title = 'NALSD classroom';
    this.user = this.auth.authUser();
  }

  logOut() {
    this.auth.setLoggedIn(false);
    this.router.navigate(['/']);
  }
}
