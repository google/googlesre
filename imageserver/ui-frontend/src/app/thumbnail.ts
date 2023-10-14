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

import {HttpClient, HttpHeaders} from '@angular/common/http';
import {Component, OnChanges, OnInit} from '@angular/core';
import {UntypedFormBuilder, FormGroup, ReactiveFormsModule, Validators} from '@angular/forms';
import {MatLegacyButtonModule as MatButtonModule} from '@angular/material/legacy-button';
import {MatLegacyChipInputEvent as MatChipInputEvent, MatLegacyChipsModule as MatChipsModule} from '@angular/material/legacy-chips';
import {MAT_LEGACY_DIALOG_DATA as MAT_DIALOG_DATA, MatLegacyDialog as MatDialog, MatLegacyDialogModule as MatDialogModule, MatLegacyDialogRef as MatDialogRef} from '@angular/material/legacy-dialog';
import {MatLegacyFormFieldModule as MatFormFieldModule} from '@angular/material/legacy-form-field';
import {MatLegacyInputModule as MatInputModule} from '@angular/material/legacy-input';
import {Router} from '@angular/router';

import {AuthService} from './authservice';
import {Download} from './download';
import {User} from './user.model';

@Component({
  selector: 'app-thumbnail',
  templateUrl: './thumbnail.ng.html',
  styleUrls: ['./thumbnail.scss'],
  providers: [Download],
})
export class Thumbnail implements OnInit {
  // search keyword
  keyword = '';
  // hashtags
  chips: string[] = [];
  // image urls for thumbnail images
  imgUrls: string[] = [];

  constructor(
      public dialog: MatDialog, private readonly httpClient: HttpClient,
      private readonly router: Router, private readonly download: Download) {}

  ngOnInit() {
    // get default thumbnail page
    this.searchImages();
  }

  openUploadForm() {
    // check if there's already a dialog open; if so, don't do anything
    if (this.dialog.openDialogs.length === 0) {
      const dialogRef = this.dialog.open(
          UploadForm, {width: '250px', data: {chips: this.chips}});

      dialogRef.afterClosed().subscribe(result => {
        console.log('The dialog was closed');
      });
    }
  }

  onImageClick(url: string) {
    const start = url.lastIndexOf('/') + 1;
    const imgName = url.substring(start);
    this.router.navigate(['/download']);
    this.download.postDownloadRequest(imgName);
  }

  searchImages() {
    const formData = new FormData();
    formData.append('keyword', this.keyword);

    const httpSearchOptions = {
      headers: new HttpHeaders({'Accept': 'multipart/form-data'})
    };

    this.httpClient
        .post('/search', formData, httpSearchOptions)
        .subscribe(
            data => {
              this.imgUrls = JSON.parse(JSON.stringify(data));
            },
            error => {
              console.error('couldn\'t post because', error);
            });
  }
}

@Component({
  selector: 'upload-form',
  templateUrl: 'upload-form.html',
  styleUrls: ['./upload-form.scss']
})
export class UploadForm {
  // hashtags
  chips: string[] = [];
  // the user interface respresents a user => currently it only has a username
  // field, but could be extended to add password, email, etc.
  user: User;
  fileList: FileList|null = null;

  constructor(
      private auth: AuthService, private fb: UntypedFormBuilder,
      private dialogRef: MatDialogRef<UploadForm>,
      private httpClient: HttpClient) {
    this.user = this.auth.authUser();
  }

  // when the usr clicks on cancel, close the dialog form
  onNoClick() {
    this.dialogRef.close();
  }

  // when the user clicks on upload, post http request to /upload endpoint
  onUploadClick() {
    this.dialogRef.close();

    if (this.fileList === null) {
      console.log('please input some file(s)');
      return;
    }

    for (let i = 0; i < this.fileList!.length; i++) {
      const formData = new FormData();

      formData.append('username', this.user.username as string);
      formData.append('hashtags', JSON.stringify(this.chips));
      formData.append('file', this.fileList![i], this.fileList![i].name);

      const httpUploadOptions = {
        headers: new HttpHeaders({'Accept': 'multipart/form-data'})
      };

      this.httpClient
          .post('/upload', formData, httpUploadOptions)
          .subscribe(
              data => {
                console.log('success!', data);
              },
              error => {
                console.error('couldn\'t post because', error);
              });
    }
  }

  onFileChange(files: FileList) {
    this.fileList = files;
  }

  addChip(event: MatChipInputEvent) {
    const input = event.input;
    const value = event.value;

    if (value.trim()) {
      this.chips.push(value.trim());
    }

    // reset the input field to be empty after we put the value into a new chip
    if (input) {
      input.value = '';
    }
  }

  removeChip(tag: string) {
    const index = this.chips.indexOf(tag);
    if (index >= 0) {
      this.chips.splice(index, 1);
    }
  }
}
