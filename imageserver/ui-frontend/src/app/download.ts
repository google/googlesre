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
import {Component} from '@angular/core';

@Component({
  selector: 'app-download',
  templateUrl: './download.ng.html',
  styleUrls: ['./download.scss']
})
export class Download {
  imgToDownload: string|null = null;
  fullImgBlob: string|null = '';

  constructor(private readonly httpClient: HttpClient) {}

  postDownloadRequest(imgName: string) {
    // because the thumbnail is served by urls, and the thumbnail's name is
    // "thumbnail_{{originalImageName}}", we can get the original image name
    // just by doing imgName.substring(10). this gets rid of the "thumbnail_"
    // prefix and returns the original image name.
    this.imgToDownload = imgName.substring(10);
    const url = '/download/' + this.imgToDownload;

    this.httpClient
        .get(url, { responseType: 'blob' })
        .subscribe(
            data => {
              var reader = new FileReader();
              reader.onloadend = function() {
                const dataURL = reader.result;
                const fullImgBlob = (dataURL as string).split(',')[1];
                const contentType = 'image/jpg';

                (document.getElementById('download-img') as HTMLImageElement)
                    .src = `data:${contentType};base64,${fullImgBlob}`;
                (document.getElementById('download-img') as HTMLImageElement)
                    .alt = "Full size image";
                (document.getElementById('img-link') as HTMLLinkElement).href =
                    `data:${contentType};base64,${fullImgBlob}`;
              };
              reader.readAsDataURL(data);
            },
            error => {
              console.error('couldn\'t post because', error);
            });
  }
}
